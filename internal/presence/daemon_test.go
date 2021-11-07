package presence

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/awilliams/wifi-presence/internal/hass"
	"github.com/awilliams/wifi-presence/internal/hostapd"
	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttAddr = flag.String("mqttAddr", "", "MQTT broker address")

func TestDaemon(t *testing.T) {
	if *mqttAddr == "" {
		t.Skip("skipping test; not given mqttAddr")
	}

	const (
		testMAC           = "FF:FF:FF:FF:FF:FF"
		otherConnectedMAC = "00:00:00:00:00:00"
		otherCfgMAC       = "AA:AA:AA:AA:AA:AA"
	)

	// Configure mock hostapd to respond to Station requests
	// with two stations.
	staResp := []hostapdtest.StationResp{
		{
			MAC:    otherConnectedMAC,
			Assoc:  true,
			Signal: 2,
		},
		{
			MAC:    testMAC,
			Assoc:  true,
			Signal: 1,
		},
	}

	h := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    42,
		SSID:       "💾",
		BSSID:      "AA:BB:CC:DD:EE:FF",
		MaxTxPower: 11,
	}, staResp)
	attachMsgs := make(chan string)
	h.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd")
		return attachMsgs
	})

	dt := newDaemonTest(t, h)

	// First subscribe to device discovery topics.
	deviceTrackerMsgs := []<-chan mqtt.Message{
		dt.subTopic(dt.topics.DeviceDiscovery(testMAC), true),
		dt.subTopic(dt.topics.DeviceDiscovery(otherCfgMAC), true),
	}
	// And state topic.
	testMACState := dt.subTopic(dt.topics.DeviceState(testMAC), true)

	// Configure Daemon to track two stations; only 1 of
	// which is returned by mock hostapd.
	trackingCfg := hass.Configuration{
		Devices: []hass.TrackConfig{
			{Name: "OTHER", MAC: otherCfgMAC},
			{Name: "Test Subject", MAC: testMAC},
		},
	}
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)

	// Ensure device discovery topics messages are received.
	for _, deviceTrackerChan := range deviceTrackerMsgs {
		select {
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for device_tracker config message")
		case err := <-dt.errs:
			t.Fatal(err)
		case msg := <-deviceTrackerChan:
			var dt hass.DeviceTracker
			if err := json.Unmarshal(msg.Payload(), &dt); err != nil {
				t.Fatalf("unable to unmarshal MQTT payload into %T: %v", dt, err)
			}
			t.Logf("msg: %v", dt)
		}
	}

	// Ensure state topic message received.
	select {
	case stateMsg := <-testMACState:
		state := string(stateMsg.Payload())
		t.Logf("device state: %q", state)
		if state != hass.PayloadHome {
			t.Fatalf("got device state %q; expected %q", state, hass.PayloadHome)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for device state message")
	}
}

func TestDaemon_Cfg(t *testing.T) {
	if *mqttAddr == "" {
		t.Skip("skipping test; not given mqttAddr")
	}

	const (
		testMAC = "FF:FF:FF:FF:FF:FF"
	)

	h := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    42,
		SSID:       "💾",
		BSSID:      "AA:BB:CC:DD:EE:FF",
		MaxTxPower: 11,
	}, nil)
	attachMsgs := make(chan string)
	h.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd")
		return attachMsgs
	})

	dt := newDaemonTest(t, h)

	// First subscribe to device discovery topic.
	dscMessages := dt.subTopic(dt.topics.DeviceDiscovery(testMAC), true)

	ensureConfigUpdate := func(want string) {
		t.Helper()
		switch want {
		case "added", "removed":
			// Ensure device discovery topic received messages.
			select {
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for device_tracker config message")
			case err := <-dt.errs:
				t.Fatal(err)
			case msg := <-dscMessages:
				if want == "added" {
					var dt hass.DeviceTracker
					if err := json.Unmarshal(msg.Payload(), &dt); err != nil {
						t.Fatalf("unable to unmarshal MQTT payload into %T: %v", dt, err)
					}
					t.Logf("msg: %+v", dt)
				} else {
					if l := len(msg.Payload()); l != 0 {
						t.Fatalf("got discovery message of length %d; expected 0", l)
					}
					t.Logf("msg: %v (removed)", msg.Payload())
				}
			}
		case "none":
			// Ensure no device discovery topic received messages.
			select {
			case <-time.After(500 * time.Millisecond):
				// OK
				t.Log("no discovery received (expected)")
			case err := <-dt.errs:
				t.Fatal(err)
			case <-dscMessages:
				t.Fatal("got unexpected discovery message")
			}
		}
	}

	// Configure Daemon to track stations.
	trackingCfg := hass.Configuration{
		Devices: []hass.TrackConfig{
			{Name: "Test Subject", MAC: testMAC},
		},
	}
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)
	ensureConfigUpdate("added")

	// Now remove device from configuration.
	dt.pubTopic(dt.topics.Config(), true, hass.Configuration{})
	ensureConfigUpdate("removed")

	// Re-add config
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)
	ensureConfigUpdate("added")

	// Re-re-add config
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)
	ensureConfigUpdate("none")

	// Final removal of device from configuration.
	dt.pubTopic(dt.topics.Config(), true, hass.Configuration{})
	ensureConfigUpdate("removed")
}

func TestDaemon_State(t *testing.T) {
	if *mqttAddr == "" {
		t.Skip("skipping test; not given mqttAddr")
	}

	const (
		testMAC = "FF:FF:FF:FF:FF:FF"
	)

	staResp := []hostapdtest.StationResp{
		{
			MAC:    testMAC,
			Assoc:  true,
			Signal: 1,
		},
	}
	h := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    42,
		SSID:       "A",
		BSSID:      "AA:BB:CC:DD:EE:FF",
		MaxTxPower: 11,
	}, staResp)
	attachMsgs := make(chan string)
	h.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd")
		return attachMsgs
	})

	dt := newDaemonTest(t, h)

	// Configure Daemon to track station.
	trackingCfg := hass.Configuration{
		Devices: []hass.TrackConfig{
			{Name: "Test Subject", MAC: testMAC},
		},
	}
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)

	// Subscribe to device's state topic.
	testMACState := dt.subTopic(dt.topics.DeviceState(testMAC), true)

	ensureState := func(want string) {
		select {
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for device state message")
		case err := <-dt.errs:
			t.Fatal(err)
		case msg := <-testMACState:
			got := string(msg.Payload())
			t.Logf("got state: %q", got)
			if got != want {
				t.Fatalf("got state %q; want %q", got, want)
			}
		}
	}

	// The station is already connected, so there
	// should be a state message.
	ensureState(hass.PayloadHome)

	// Send disconnect event.
	select {
	case attachMsgs <- fmt.Sprintf("AP-STA-DISCONNECTED %s", testMAC):
		t.Log("sent disconnect event")
	case <-time.After(time.Second):
		t.Fatal("timeout sending disconnect event")
	}
	ensureState(hass.PayloadNotHome)

	// Send connect event.
	select {
	case attachMsgs <- fmt.Sprintf("AP-STA-CONNECTED %s", testMAC):
		t.Log("sent connect event")
	case <-time.After(time.Second):
		t.Fatal("timeout sending connect event")
	}
	ensureState(hass.PayloadHome)
}

func TestDaemon_MultiAPs(t *testing.T) {
	if *mqttAddr == "" {
		t.Skip("skipping test; not given mqttAddr")
	}

	const (
		testMAC = "FF:FF:FF:FF:FF:FF"
		h1SSID  = "ssid-1"
		h2SSID  = "ssid-2"
	)

	// testMAC will initially be connected to h1.

	staResp := []hostapdtest.StationResp{
		{
			MAC:    testMAC,
			Assoc:  true,
			Signal: 1,
		},
	}
	h1 := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    42,
		SSID:       h1SSID,
		BSSID:      "AA:BB:CC:DD:EE:FF",
		MaxTxPower: 11,
	}, staResp)
	h1attachMsgs := make(chan string)
	h1.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd 1")
		return h1attachMsgs
	})

	h2 := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    43,
		SSID:       h2SSID,
		BSSID:      "FF:BB:CC:DD:EE:FF",
		MaxTxPower: 12,
	}, nil)
	h2attachMsgs := make(chan string)
	h2.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd 2")
		return h2attachMsgs
	})

	dt := newDaemonTest(t, h1, h2)

	// Configure Daemon to track station.
	trackingCfg := hass.Configuration{
		Devices: []hass.TrackConfig{
			{Name: "Test Subject", MAC: testMAC},
		},
	}
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)

	// Subscribe to device's state and attrs topic.
	testMACState := dt.subTopic(dt.topics.DeviceState(testMAC), true)
	testMACAttrs := dt.subTopic(dt.topics.DeviceJSONAttrs(testMAC), true)

	ensureState := func(wantState, wantSSID string) {
		select {
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for device state message")
		case err := <-dt.errs:
			t.Fatal(err)
		case msg := <-testMACState:
			got := string(msg.Payload())
			t.Logf("got state: %q", got)
			if got != wantState {
				t.Fatalf("got state %q; want %q", got, wantState)
			}
		}

		select {
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for device attrs message")
		case err := <-dt.errs:
			t.Fatal(err)
		case msg := <-testMACAttrs:
			var attrs hass.Attrs
			if err := json.Unmarshal(msg.Payload(), &attrs); err != nil {
				t.Fatal(err)
			}

			if got := attrs.SSID; got != wantSSID {
				t.Fatalf("got SSID %q; want %q", got, wantSSID)
			}
			t.Logf("got SSID %q", attrs.SSID)
		}
	}
	sendEvent := func(hap chan string, event string) {
		select {
		case hap <- event:
			t.Logf("sent %q event", event)
		case err := <-dt.errs:
			t.Fatal(err)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout sending %q event", event)
		}
	}

	// The station is already connected, so there
	// should be a state message.
	ensureState(hass.PayloadHome, h1SSID)

	// Send disconnect event.
	sendEvent(h1attachMsgs, fmt.Sprintf("AP-STA-DISCONNECTED %s", testMAC))
	ensureState(hass.PayloadNotHome, h1SSID)

	// Send connect event.
	sendEvent(h2attachMsgs, fmt.Sprintf("AP-STA-CONNECTED %s", testMAC))
	ensureState(hass.PayloadHome, h2SSID)

	// Send connect event on h1 AP.
	sendEvent(h1attachMsgs, fmt.Sprintf("AP-STA-CONNECTED %s", testMAC))
	ensureState(hass.PayloadHome, h1SSID)

	// Send duplicate connect event on h1 AP.
	// Should be ignored.
	sendEvent(h1attachMsgs, fmt.Sprintf("AP-STA-CONNECTED %s", testMAC))

	// Ensure duplicate connect is ignored.
	select {
	case <-time.After(250 * time.Millisecond):
		t.Logf("no state message received after duplicate event")
	case err := <-dt.errs:
		t.Fatal(err)
	case msg := <-testMACState:
		got := string(msg.Payload())
		t.Fatalf("got state message: %q; expected no update", got)
	}
}

func TestDaemon_DelayedDisconnect(t *testing.T) {
	if *mqttAddr == "" {
		t.Skip("skipping test; not given mqttAddr")
	}

	const (
		testMAC = "FF:FF:FF:FF:FF:FF"
		h1SSID  = "ssid-1"
		h2SSID  = "ssid-2"
	)

	// testMAC will not be initially connected.

	h1 := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    42,
		SSID:       h1SSID,
		BSSID:      "AA:BB:CC:DD:EE:FF",
		MaxTxPower: 11,
	}, nil)
	h1attachMsgs := make(chan string)
	h1.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd 1")
		return h1attachMsgs
	})

	h2 := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "Enabled",
		Channel:    43,
		SSID:       h2SSID,
		BSSID:      "FF:BB:CC:DD:EE:FF",
		MaxTxPower: 12,
	}, nil)
	h2attachMsgs := make(chan string)
	h2.OnAttach(func() <-chan string {
		t.Log("Attached to hostapd 2")
		return h2attachMsgs
	})

	dt := newDaemonTest(t, h1, h2)

	// Configure Daemon to track station.
	trackingCfg := hass.Configuration{
		Devices: []hass.TrackConfig{
			{Name: "Test Subject", MAC: testMAC},
		},
	}
	dt.pubTopic(dt.topics.Config(), true, trackingCfg)

	// Subscribe to device's state and attrs topic.
	testMACState := dt.subTopic(dt.topics.DeviceState(testMAC), true)
	testMACAttrs := dt.subTopic(dt.topics.DeviceJSONAttrs(testMAC), true)

	ensureState := func(wantState, wantSSID string) {
		t.Helper()
		select {
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for device state message")
		case err := <-dt.errs:
			t.Fatal(err)
		case msg := <-testMACState:
			got := string(msg.Payload())
			t.Logf("got state: %q", got)
			if got != wantState {
				t.Fatalf("got state %q; want %q", got, wantState)
			}
		}

		if wantSSID == "" {
			return
		}

		select {
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for device attrs message")
		case err := <-dt.errs:
			t.Fatal(err)
		case msg := <-testMACAttrs:
			var attrs hass.Attrs
			if err := json.Unmarshal(msg.Payload(), &attrs); err != nil {
				t.Fatal(err)
			}

			if got := attrs.SSID; got != wantSSID {
				t.Fatalf("got SSID %q; want %q", got, wantSSID)
			}
			t.Logf("got SSID %q", attrs.SSID)
		}
	}

	sendEvent := func(hap chan string, event string) {
		select {
		case hap <- event:
			t.Logf("sent %q event", event)
		case err := <-dt.errs:
			t.Fatal(err)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout sending %q event", event)
		}
	}

	// No SSID for initial disconnected state.
	ensureState(hass.PayloadNotHome, "")

	// Send h1 connect event.
	sendEvent(h1attachMsgs, fmt.Sprintf("AP-STA-CONNECTED %s", testMAC))
	ensureState(hass.PayloadHome, h1SSID)

	// Simulate when a station connects to another AP,
	// and then has a delayed disconnect event from originally connected AP.

	// Send h2 connect event.
	sendEvent(h2attachMsgs, fmt.Sprintf("AP-STA-CONNECTED %s", testMAC))
	ensureState(hass.PayloadHome, h2SSID)

	// Send delayed disconnect event.
	// Should be ignored.
	sendEvent(h1attachMsgs, fmt.Sprintf("AP-STA-DISCONNECTED %s", testMAC))

	// Ensure delayed disconnect is ignored.
	select {
	case <-time.After(250 * time.Millisecond):
		t.Logf("no state message received after delayed disconnect")
	case err := <-dt.errs:
		t.Fatal(err)
	case msg := <-testMACState:
		got := string(msg.Payload())
		t.Fatalf("got state message: %q; expected no update", got)
	}
}

func newDaemonTest(t *testing.T, hapHandlers ...*hostapdtest.Handler) *daemonTest {
	if *mqttAddr == "" {
		t.Skip("skipping test; not given mqttAddr")
	}
	t.Helper()

	tmpDir := tempDir(t)

	var (
		uid    = time.Now().UnixNano() / 1000
		errs   = make(chan error, 1+len(hapHandlers))
		topics = hass.MQTTTopics{
			Name:       t.Name(),
			Prefix:     fmt.Sprintf("%s-%d", t.Name(), uid),
			HASSPrefix: fmt.Sprintf("homeassistant-%s-%d", t.Name(), uid),
		}
	)

	// Create MQTT client used for pub/sub by the test itself.
	opts := mqtt.NewClientOptions()
	opts.SetClientID(fmt.Sprintf("%s-%d", t.Name(), uid))
	opts.AddBroker(*mqttAddr)
	opts.SetCleanSession(true)
	opts.SetOrderMatters(true)

	mc := mqtt.NewClient(opts)
	tkn := mc.Connect()
	if !tkn.WaitTimeout(time.Second) {
		t.Fatal("mqtt connect timeout")
	}
	t.Cleanup(func() { mc.Disconnect(1500) })

	// Setup mock hostapd instances.
	hostapdAddrs := make([]string, len(hapHandlers))
	for i, h := range hapHandlers {
		thap, err := hostapdtest.NewHostAPD(path.Join(tmpDir, fmt.Sprintf("hap-%02d", i+1)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { thap.Close() })

		hostapdAddrs[i] = thap.Addr
		go func(h *hostapdtest.Handler) {
			if err := thap.Serve(h); err != nil {
				errs <- fmt.Errorf("hostapdtest error: %w", err)
			}
		}(h)
	}

	// Setup hass.MQTT instance.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hm, err := hass.NewMQTT(ctx, hass.MQTTOpts{
		BrokerAddr:      *mqttAddr,
		ClientID:        fmt.Sprintf("%s-wp-%d", t.Name(), time.Now().UnixNano()),
		APName:          topics.Name,
		TopicPrefix:     topics.Prefix,
		DiscoveryPrefix: topics.HASSPrefix,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hm.Close() })

	// Setup hostapd instances, which connect to the test hostapds.
	var daemonOpts []Opt
	for _, addr := range hostapdAddrs {
		hap, err := hostapd.NewClient(tmpDir, addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { hap.Close() })
		daemonOpts = append(daemonOpts, WithHostAPD(hap))
	}

	// Finally, create the Daemon instance.
	daemonOpts = append(daemonOpts, WithHassOpt(hm))
	daemonOpts = append(daemonOpts, WithDebounce(0)) // Handle disconnect events immediately.
	daemonOpts = append(daemonOpts, WithHASSAutodiscovery(true))
	d, err := NewDaemon(daemonOpts...)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := d.Run(ctx); err != nil {
			errs <- fmt.Errorf("daemon error: %w", err)
		}
	}()

	return &daemonTest{
		t:      t,
		errs:   errs,
		topics: topics,
		mc:     mc,
	}
}

type daemonTest struct {
	t      *testing.T
	errs   <-chan error
	topics hass.MQTTTopics
	mc     mqtt.Client
}

func (d *daemonTest) subTopic(topic string, ignoreRetained bool) <-chan mqtt.Message {
	msgs := make(chan mqtt.Message, 1)
	tkn := d.mc.Subscribe(topic, 2, func(_ mqtt.Client, msg mqtt.Message) {
		defer msg.Ack()
		if !(ignoreRetained && msg.Retained()) {
			msgs <- msg
		}
	})
	if !tkn.WaitTimeout(time.Second) {
		d.t.Fatalf("subscribe %q timeout", topic)
	}
	d.t.Cleanup(func() { d.mc.Unsubscribe(topic) })
	return msgs
}

func (d *daemonTest) pubTopic(topic string, retain bool, jsonPayload interface{}) {
	payload, err := json.Marshal(jsonPayload)
	if err != nil {
		d.t.Fatal(err)
	}
	tkn := d.mc.Publish(topic, 2, retain, payload)
	if !tkn.WaitTimeout(time.Second) {
		d.t.Fatalf("mqtt publish %q timeout", topic)
	}
}

func tempDir(t *testing.T) string {
	// Alternative to t.TempDir() used to generate a shorter
	// temp dir path. These dirs may be used for sockets,
	// and there may be limitations on socket path lengths.
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("tempDir RemoveAll cleanup: %v", err)
		}
	})
	return tempDir
}
