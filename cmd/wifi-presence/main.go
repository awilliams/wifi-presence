package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/awilliams/wifi-presence/pkg/hostapd"
	"github.com/awilliams/wifi-presence/pkg/presence"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/sync/errgroup"
)

const (
	appName               = "wifi-presence"
	defaultHostapdSockDir = "/var/run/hostapd/"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel context when a terminating signal is received.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("Received signal %q, exiting...", <-sigs)
		cancel()
	}()

	if err := run(ctx, appName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

const helpTxt = `
About:
wifi-presence monitors a WiFi access point (AP) and publishes events to an MQTT topic
when clients connect or disconnect. The debounce option can be used to delay sending
a disconnect event. This is useful to prevent events from clients that quickly
disconnect then re-connect.

hostapd/wpa_supplicant:
wifi-presence requires hostapd running with control interface(s) enabled.
The hostapd option is 'ctrl_interface'. More information:
https://w1.fi/cgit/hostap/plain/hostapd/hostapd.conf

The -hostapd.socks option should correspond to the socket
locations defined by 'ctrl_interface'. Multiple sockets
can be monitored (one socket per radio is created by hostapd).

MQTT:
The prefix of the MQTT topic is configurable using
options defined above:

$mqtt.prefix/$apName/$clientMAC

The body of the connect disconnect messages is JSON. Example:

%s

The program will publish a status message when starting and exiting
to the following topic:

$mqtt.prefix/$apName/status

The body of the message is JSON. Example:

%s
`

// run executes the wifi-presence program. It stops when an error occurs or
// the context is cancelled.
func run(ctx context.Context, appName string) error {
	// Parse and validate program arguments.

	hostName, _ := os.Hostname()
	args := struct {
		apName       string
		hostapdSocks string
		mqttAddr     string
		mqttID       string
		mqttPrefix   string
		mqttUsername string
		mqttPassword string
		debounce     time.Duration
		verbose      bool
		version      bool
	}{
		apName: hostName,
		hostapdSocks: func() string {
			return strings.Join(
				findSockets(defaultHostapdSockDir),
				string(os.PathListSeparator),
			)
		}(),
		mqttID: func() string {
			if hostName == "" {
				return appName
			}
			return fmt.Sprintf("%s.%s", appName, hostName)
		}(),
		mqttPrefix: appName,
		debounce:   10 * time.Second,
		verbose:    false,
		version:    false,
	}

	flag.StringVar(&args.apName, "apName", args.apName, "Access point name")
	flag.StringVar(&args.hostapdSocks, "hostapd.socks", args.hostapdSocks, fmt.Sprintf("Hostapd control interface socket(s). Separate multiple paths by %q", os.PathListSeparator))
	flag.StringVar(&args.mqttAddr, "mqtt.addr", args.mqttAddr, "MQTT broker address, e.g \"tcp://mqtt.broker:1883\"")
	flag.StringVar(&args.mqttID, "mqtt.id", args.mqttID, "MQTT client ID")
	flag.StringVar(&args.mqttPrefix, "mqtt.prefix", args.mqttPrefix, "MQTT topic prefix")
	flag.StringVar(&args.mqttUsername, "mqtt.username", args.mqttUsername, "MQTT username (optional)")
	flag.StringVar(&args.mqttPassword, "mqtt.password", args.mqttPassword, "MQTT password (optional)")
	flag.DurationVar(&args.debounce, "debounce", args.debounce, "Time to wait until considering a station disconnected. Examples: 5s, 1m")
	flag.BoolVar(&args.verbose, "verbose", args.verbose, "Verbose logging")
	flag.BoolVar(&args.verbose, "v", args.verbose, "Verbose logging (alias)")
	flag.BoolVar(&args.version, "version", args.version, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n\nOptions:\n", appName)
		flag.PrintDefaults()

		connectEx, _ := json.MarshalIndent(presence.StationEvent{
			AP:        args.apName,
			SSID:      "wifi-name",
			BSSID:     "XX:XX:XX:XX:XX:XX",
			MAC:       presence.MAC{0xAB, 0xCD, 0xEF, 0x12, 0x34, 0x56},
			Action:    presence.ActionConnect,
			Timestamp: time.Now(),
		}, "", "  ")
		statusEx, _ := json.MarshalIndent(status{
			Status:    statusOnline,
			Timestamp: time.Now(),
		}, "", "  ")

		fmt.Fprintf(os.Stderr, helpTxt, string(connectEx), string(statusEx))
	}
	flag.Parse()

	if args.version {
		fmt.Printf("wifi-presence v%s\n", version)
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

	// Set all logging to /dev/null unless verbose flag was set.
	if !args.verbose {
		log.SetOutput(ioutil.Discard)
	}

	// Create MQTT client.

	mqttErr := make(chan error, 1)
	mqttOpts := func(o *mqttClientOptions) {
		o.AddBroker(args.mqttAddr)
		o.SetClientID(args.mqttID)
		o.SetAPName(args.apName)
		o.SetTopicPrefix(args.mqttPrefix)
		o.SetCleanSession(false)
		o.SetConnectRetry(false)
		o.SetAutoReconnect(false)
		o.SetKeepAlive(2 * time.Minute)
		if args.mqttUsername != "" || args.mqttPassword != "" {
			o.SetCredentialsProvider(mqtt.CredentialsProvider(func() (username string, password string) {
				return args.mqttUsername, args.mqttPassword
			}))
		}
		o.SetConnectionLostHandler(func(c mqtt.Client, err error) {
			mqttErr <- fmt.Errorf("MQTT connection lost: %w", err)
		})
	}

	mc, err := newMQTTClient(ctx, mqttOpts)
	if err != nil {
		return err
	}
	// Use MQTT will to publish online & offline messages.
	// Offline message will only be automatically published if MQTT
	// connection is broken. So we must explicitly publish it during a
	// "normal" shutdown.
	if err := mc.publishWill(ctx, statusOnline); err != nil {
		return err
	}
	defer func() {
		// Cannot use original context since it may have already
		// been cancelled.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = mc.publishWill(ctx, statusOffline)
		cancel()
		mc.close()
	}()

	// Setup wait group. Run main application logic
	// for each hostapd socket.

	eg, egCtx := errgroup.WithContext(ctx)

	// Stop on first MQTT error, e.g. connection lost.
	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return nil
		case err := <-mqttErr:
			return err
		}
	})

	// Connect to and monitor each hostapd control interface socket.
	for _, ctrlSock := range strings.Split(args.hostapdSocks, string(os.PathListSeparator)) {
		hostapdClient, err := hostapd.NewClient(ctrlSock)
		if err != nil {
			return fmt.Errorf("unable to connect to hostapd control socket %q: %w", ctrlSock, err)
		}

		log.Printf("Connected to hostapd interface: %q", ctrlSock)

		a, err := newApp(args.apName, args.debounce, mc, hostapdClient)
		if err != nil {
			return err
		}

		eg.Go(func() error {
			defer hostapdClient.Close()
			return a.run(egCtx)
		})
	}

	return eg.Wait()
}

// findSockets returns paths to all Unix domain sockets
// in the given directory.
func findSockets(dir string) []string {
	var sockets []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Match if file is a Unix domain socket.
		if info.Mode()&os.ModeSocket != 0 {
			sockets = append(sockets, path)
		}
		return nil
	})
	return sockets
}
