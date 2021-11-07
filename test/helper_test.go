package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/awilliams/wifi-presence/internal/hass"
	"github.com/awilliams/wifi-presence/internal/hostapd"
	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttAddr = flag.String("mqttAddr", "", "Test MQTT broker address, e.g. tcp://localhost:1883")

func newIntTest(t *testing.T, hostAPHandlers []*hostapdtest.Handler) *intTest {
	wpErrs := make(chan error, 1)
	hapErrs := make(chan error, len(hostAPHandlers))

	uid := time.Now().UnixNano() / 1000

	it := intTest{
		t:    t,
		mqtt: mqttClient(t),
		topics: hass.MQTTTopics{
			Name:       fmt.Sprintf("%s-%d", t.Name(), uid),
			Prefix:     fmt.Sprintf("%s-%d", t.Name(), uid),
			HASSPrefix: fmt.Sprintf("homeassistant-%s-%d", t.Name(), uid),
		},
		startWP: make(chan struct{}),
		hapErrs: hapErrs,
		wpErrs:  wpErrs,
	}

	hostAPDs := make([]*hostapdtest.HostAPD, 0, len(hostAPHandlers))

	for i, h := range hostAPHandlers {
		hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), fmt.Sprintf("hap-%02d", i)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { hostapd.Close() })

		hostAPDs = append(hostAPDs, hostapd)
		go func(h *hostapdtest.Handler) {
			if err := hostapd.Serve(h); err != nil {
				hapErrs <- fmt.Errorf("hostAPD error: %w", err)
			}
		}(h)
	}

	wp := wifiPresenceExec{
		apName:            fmt.Sprintf("%s-%d", t.Name(), uid),
		debounce:          0,
		hassAutodiscovery: true,
		hassPrefix:        it.topics.HASSPrefix,
		hostapdSocks: func() []string {
			socks := make([]string, len(hostAPDs))
			for i, m := range hostAPDs {
				socks[i] = m.Addr
			}
			return socks
		}(),
		mqttAddr:   *mqttAddr,
		mqttID:     fmt.Sprintf("%s.wifi-presence.%d", t.Name(), time.Now().UnixNano()),
		mqttPrefix: it.topics.Prefix,
		verbose:    true,
	}

	f := wp.exec(t)
	go func() {
		<-it.startWP
		if err := f(); err != nil {
			wpErrs <- fmt.Errorf("wifi-presence error: %w", err)
			return
		}
		wpErrs <- nil
	}()

	return &it
}

type intTest struct {
	t      *testing.T
	mqtt   mqtt.Client
	topics hass.MQTTTopics

	startWP chan struct{}
	hapErrs <-chan error
	wpErrs  <-chan error
}

func (i *intTest) startWifiPresence() {
	i.t.Helper()
	statusSub := i.subTopic(i.topics.Will(), true)
	close(i.startWP)

	// Wait for status message.
	msg := i.waitMessage(statusSub)
	if string(msg.Payload()) != "online" {
		i.t.Fatalf("unexpected status message: %q", string(msg.Payload()))
	}
}

func (i *intTest) waitMessage(c <-chan mqtt.Message) mqtt.Message {
	i.t.Helper()
	select {
	case err := <-i.hapErrs:
		i.t.Fatal(err)
	case err := <-i.wpErrs:
		i.t.Fatal(err)
	case msg := <-c:
		return msg
	case <-time.After(time.Second):
		i.t.Fatal("timeout waiting for MQTT message")
	}
	return nil
}

func (i *intTest) subTopic(topic string, ignoreRetained bool) <-chan mqtt.Message {
	msgs := make(chan mqtt.Message, 1)
	tkn := i.mqtt.Subscribe(topic, 2, func(_ mqtt.Client, msg mqtt.Message) {
		defer msg.Ack()
		if !(ignoreRetained && msg.Retained()) {
			msgs <- msg
		}
	})
	if !tkn.WaitTimeout(time.Second) {
		i.t.Fatal("subscribe timeout")
	}
	i.t.Cleanup(func() { i.mqtt.Unsubscribe(topic) })
	i.t.Logf("subscribed to topic: %q", topic)
	return msgs
}

func (i *intTest) pubTopic(topic string, retain bool, jsonPayload interface{}) {
	payload, err := json.Marshal(jsonPayload)
	if err != nil {
		i.t.Fatal(err)
	}
	tkn := i.mqtt.Publish(topic, 2, retain, payload)
	if !tkn.WaitTimeout(time.Second) {
		i.t.Fatal("mqtt publish timeout")
	}
}

type wifiPresenceExec struct {
	apName            string
	debounce          time.Duration
	hassAutodiscovery bool
	hassPrefix        string
	hostapdSocks      []string
	mqttAddr          string
	mqttID            string
	mqttPrefix        string
	verbose           bool
}

// Exec returns a function that starts the wifi-presence binary.
// This is done to allow for setup tasks to call t.Fatal, and then
// for the running of the binary to be done in a separate goroutine.
func (w wifiPresenceExec) exec(t *testing.T) func() error {
	if w.mqttAddr == "" {
		w.mqttAddr = "tcp://localhost:1883"
	}
	if w.mqttID == "" {
		w.mqttID = fmt.Sprintf("%s.wifi-presence.%d", t.Name(), time.Now().UnixNano())
	}
	if w.mqttPrefix == "" {
		w.mqttPrefix = t.Name()
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	bin := filepath.Join(t.TempDir(), "wifi-presence")
	// TODO: add -race
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "../cmd/wifi-presence")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build command %q failed:\n%s", buildCmd.String(), string(output))
	}

	// Log stdout/stderr.
	var out bytes.Buffer
	t.Cleanup(func() {
		t.Log("wifi-presence output:\n", out.String())
	})

	args := []string{
		"-apName", w.apName,
		"-sockDir", t.TempDir(),
		"-debounce", w.debounce.String(),
		fmt.Sprintf("-hass.autodiscovery=%s", strconv.FormatBool(w.hassAutodiscovery)),
		"-hass.prefix", w.hassPrefix,
		"-hostapd.socks", strings.Join(w.hostapdSocks, string(filepath.ListSeparator)),
		"-mqtt.addr", w.mqttAddr,
		"-mqtt.id", w.mqttID,
		"-mqtt.prefix", w.mqttPrefix,
		fmt.Sprintf("-verbose=%s", strconv.FormatBool(w.verbose)),
	}

	runCmd := exec.Command(bin, args...)
	runCmd.Stdout = &out
	runCmd.Stderr = &out

	return func() error {
		err := runCmd.Run()
		if err == nil {
			return nil
		}
		exitErr := new(exec.ExitError)
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 125 {
			// This is a special error code returned when
			// the (mock) hostapd sends a termination event.
			return hostapd.ErrTerminating
		}
		return fmt.Errorf("run command %q failed: %v", runCmd.String(), err)
	}
}

func mqttClient(t *testing.T) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.SetClientID(fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()))
	opts.AddBroker(*mqttAddr)
	opts.SetCleanSession(true)
	opts.SetOrderMatters(true)

	c := mqtt.NewClient(opts)
	tkn := c.Connect()
	if !tkn.WaitTimeout(time.Second) {
		t.Fatal("mqtt connect timeout")
	}
	t.Cleanup(func() { c.Disconnect(500) })

	return c
}
