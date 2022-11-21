// wifi-presence executable
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/awilliams/wifi-presence/internal/hass"
	"github.com/awilliams/wifi-presence/internal/hostapd"
	"github.com/awilliams/wifi-presence/internal/presence"

	"golang.org/x/sync/errgroup"
)

const (
	appName               = "wifi-presence"
	defaultHostapdSockDir = "/var/run/hostapd/"
)

const helpTxt = `
About:
wifi-presence monitors a WiFi access point (AP) and publishes client connect and
disconnect events to an MQTT topic.

hostapd/wpa_supplicant:
wifi-presence requires hostapd running with control interface(s) enabled.
The hostapd option is 'ctrl_interface'. More information:
https://w1.fi/cgit/hostap/plain/hostapd/hostapd.conf

The wifi-presence -hostapd.socks option should correspond to the socket
locations defined by 'ctrl_interface'. Multiple sockets can be monitored
(one socket per radio is created by hostapd).

MQTT:
wifi-presence publishes and subscribes to an MQTT broker.
The -mqtt.prefix flag can be used to change the topic prefix,
along with -hass.prefix for Home Assistant's topic prefix.
The provided -apName is also used as part of some topics.
It will be modified for compatibility with MQTT topics
(downcased, spaces removed, etc).

The following topics are used:

  * <PREFIX>/<AP_NAME>/status
  The status of wifi-presence (online / offline).

  * <PREFIX>/config
  wifi-presence subscribes to this topic for configuration updates.

  * <HASS_PREFIX>/device_tracker/<AP_NAME>/<MAC>/config
  If -hass.autodiscovery is enabled, then all configured devices will be published
  to these topics (based on their MAC address). Home Assistant subscribes to these
  topics and registers/unregisters entities accordingly based on messages received.

  * <PREFIX>/station/<AP_NAME>/<MAC>/state
  The state of a device (home / not_home) is published to these topics.

  * <PREFIX>/station/<AP_NAME>/<MAC>/attrs
  A JSON object with device attributes (SSID, BSSID, etc) is published to these topics.
`

func main() {
	// Main program context which terminates when terminating signal is received.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, appName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)

		var unknownCmd hostapd.ErrUnknownCmd
		switch {
		case errors.Is(err, hostapd.ErrTerminating):
			// Special exit code to indicate that hostapd indicated that
			// the process should exit.
			os.Exit(125)
		case errors.As(err, &unknownCmd):
			fmt.Fprintln(os.Stderr, "This error typically happens when the full version of 'hostapd' is not installed. See https://github.com/awilliams/wifi-presence/#hostapd-full-version for more information.")
		}

		os.Exit(1)
	}
}

// run executes the wifi-presence program. It stops when an error occurs or
// the context is cancelled.
func run(ctx context.Context, appName string) error {
	// Parse and validate program arguments.

	hostName, _ := os.Hostname()
	args := struct {
		apName            string
		sockDir           string
		hostapdSocks      string
		mqttAddr          string
		mqttID            string
		mqttPrefix        string
		mqttUsername      string
		mqttPassword      string
		hassAutodiscovery bool
		hassPrefix        string
		debounce          time.Duration
		verbose           bool

		version  bool
		moreHelp bool
	}{
		apName:  hostName,
		sockDir: os.TempDir(),
		hostapdSocks: func() string {
			return strings.Join(
				findUnixSockets(defaultHostapdSockDir),
				string(os.PathListSeparator),
			)
		}(),
		mqttID: func() string {
			if hostName == "" {
				return appName
			}
			return fmt.Sprintf("%s.%s", appName, hostName)
		}(),
		mqttPrefix:        appName,
		hassAutodiscovery: true,
		hassPrefix:        "homeassistant",
		debounce:          10 * time.Second,
		verbose:           false,
		version:           false,
		moreHelp:          false,
	}

	flag.StringVar(&args.apName, "apName", args.apName, "Access point name")
	flag.StringVar(&args.sockDir, "sockDir", args.sockDir, "Directory for local socket(s)")
	flag.StringVar(&args.hostapdSocks, "hostapd.socks", args.hostapdSocks, fmt.Sprintf("Hostapd control interface socket(s). Separate multiple paths by %q", os.PathListSeparator))
	flag.StringVar(&args.mqttAddr, "mqtt.addr", args.mqttAddr, "MQTT broker address, e.g \"tcp://mqtt.broker:1883\"")
	flag.StringVar(&args.mqttID, "mqtt.id", args.mqttID, "MQTT client ID")
	flag.StringVar(&args.mqttPrefix, "mqtt.prefix", args.mqttPrefix, "MQTT topic prefix")
	flag.StringVar(&args.mqttUsername, "mqtt.username", args.mqttUsername, "MQTT username (optional)")
	flag.StringVar(&args.mqttPassword, "mqtt.password", args.mqttPassword, "MQTT password (optional)")
	flag.BoolVar(&args.hassAutodiscovery, "hass.autodiscovery", args.hassAutodiscovery, "Enable Home Assistant MQTT autodiscovery")
	flag.StringVar(&args.hassPrefix, "hass.prefix", args.hassPrefix, "Home Assistant MQTT topic prefix")
	flag.DurationVar(&args.debounce, "debounce", args.debounce, "Time to wait until considering a station disconnected. Examples: 5s, 1m")
	flag.BoolVar(&args.verbose, "verbose", args.verbose, "Verbose logging")
	flag.BoolVar(&args.verbose, "v", args.verbose, "Verbose logging (alias)")
	flag.BoolVar(&args.version, "version", args.version, "Print version and exit")
	flag.BoolVar(&args.moreHelp, "help", args.moreHelp, "Print detailed help message")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n\nOptions:\n", appName)
		flag.PrintDefaults()

		if args.moreHelp {
			fmt.Fprint(os.Stderr, helpTxt)
		}
	}
	flag.Parse()

	if args.version {
		fmt.Printf("wifi-presence v%s\n", version)
		return nil
	}
	if args.moreHelp {
		flag.Usage()
		return nil
	}

	if args.apName == "" {
		return errors.New("apName cannot be blank")
	}
	if args.hostapdSocks == "" {
		return errors.New("hostapd.socks cannot be blank")
	}
	if args.mqttAddr == "" {
		return errors.New("mqtt.addr cannot be blank")
	}
	if args.mqttID == "" {
		return errors.New("mqtt.id cannot be blank")
	}
	if args.mqttPrefix == "" {
		return errors.New("mqtt.prefix cannot be blank")
	}

	if args.hassAutodiscovery {
		if args.hassPrefix == "" {
			return errors.New("hass.prefix cannot be blank when autodiscovery is enabled")
		}
	}

	// Set all logging to /dev/null unless verbose flag was set.
	if !args.verbose {
		log.SetOutput(io.Discard)
	}

	// Create MQTT client.

	mqttOpts := hass.MQTTOpts{
		APName:          args.apName,
		BrokerAddr:      args.mqttAddr,
		ClientID:        args.mqttID,
		Username:        args.mqttUsername,
		Password:        args.mqttPassword,
		TopicPrefix:     args.mqttPrefix,
		DiscoveryPrefix: args.hassPrefix,
	}
	mqtt, err := hass.NewMQTT(ctx, mqttOpts)
	if err != nil {
		return err
	}

	statusCtx, statusCancel := context.WithTimeout(ctx, 2*time.Second)
	defer statusCancel()
	if err := mqtt.StatusOnline(statusCtx); err != nil {
		return err
	}
	defer func() {
		// Cannot use main context since it may have already been cancelled.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = mqtt.StatusOffline(ctx)
		cancel()

		mqtt.Close()
	}()

	sockets := strings.Split(args.hostapdSocks, string(os.PathListSeparator))
	hostapds := make([]*hostapd.Client, 0, len(sockets))

	// Connect to each hostapd control interface socket.
	for _, ctrlSock := range sockets {
		hostapdClient, err := hostapd.NewClient(args.sockDir, ctrlSock)
		if err != nil {
			return fmt.Errorf("unable to connect to hostapd control socket %q: %w", ctrlSock, err)
		}
		defer hostapdClient.Close()

		hostapds = append(hostapds, hostapdClient)
	}

	var opts []presence.Opt
	opts = append(opts, presence.WithAPName(args.apName))
	opts = append(opts, presence.WithHassOpt(mqtt))
	opts = append(opts, presence.WithLogger(log.Default()))
	opts = append(opts, presence.WithDebounce(args.debounce))
	opts = append(opts, presence.WithHASSAutodiscovery(args.hassAutodiscovery))
	for _, hap := range hostapds {
		opts = append(opts, presence.WithHostAPD(hap))
	}

	d, err := presence.NewDaemon(opts...)
	if err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error { return mqtt.OnConnectionLost(egCtx) })
	eg.Go(func() error { return d.Run(egCtx) })

	return eg.Wait()
}

// findUnixSockets returns paths to all Unix domain sockets
// in the given directory.
func findUnixSockets(dir string) []string {
	var sockets []string
	_ = filepath.WalkDir(dir, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// The "global" control interface cannot be used for wifi-presence
		// purposes, e.g. getting station information.
		if filepath.Base(path) == "global" {
			return nil
		}

		// Test if file is a Unix domain socket.
		if de.Type()&os.ModeSocket != 0 {
			sockets = append(sockets, path)
		}
		return nil
	})
	return sockets
}
