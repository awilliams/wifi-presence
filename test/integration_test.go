package integration_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/awilliams/wifi-presence/internal/hass"
	"github.com/awilliams/wifi-presence/internal/hostapd"
	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"
)

func TestWifiPresence(t *testing.T) {
	if *mqttAddr == "" {
		t.Skip("skipping since no -mqttAddr is set")
	}
	const (
		station1MAC = "FF:FF:FF:FF:FF:FF"
		station2MAC = "BE:EF:00:00:00:01"
		bssid1      = "FF:FF:FF:FF:FF:01"
		bssid2      = "AA:FF:FF:FF:FF:01"
	)

	hap1 := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "ENABLED",
		Channel:    42,
		SSID:       "My WiFi",
		BSSID:      bssid1,
		MaxTxPower: 11,
	}, []hostapdtest.StationResp{
		{
			MAC:    station1MAC,
			Assoc:  true,
			Signal: 22,
		},
	})
	hap1Events := make(chan string)
	hap1.OnAttach(func() <-chan string {
		t.Log("hostapd1 attached")
		return hap1Events
	})

	hap2 := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		State:      "ENABLED",
		Channel:    43,
		SSID:       "5G WIFI",
		BSSID:      bssid2,
		MaxTxPower: 11,
	}, nil)
	hap2Events := make(chan string)
	hap2.OnAttach(func() <-chan string {
		t.Log("hostapd2 attached")
		return hap2Events
	})

	it := newIntTest(t, []*hostapdtest.Handler{hap1, hap2})

	// Subscribe to relevant topics for stations 1 & 2.
	var (
		station1ConfigSub = it.subTopic(it.topics.DeviceDiscovery(station1MAC), true)
		station2ConfigSub = it.subTopic(it.topics.DeviceDiscovery(station2MAC), true)
		station1StateSub  = it.subTopic(it.topics.DeviceState(station1MAC), true)
		station2StateSub  = it.subTopic(it.topics.DeviceState(station2MAC), true)
	//	station1StateAttrs = it.subTopic(it.topics.DeviceJSONAttrs(station1MAC), true)
	//	station2StateAttrs = it.subTopic(it.topics.DeviceJSONAttrs(station2MAC), true)
	)

	trackingCfg := hass.Configuration{
		Devices: []hass.TrackConfig{
			{Name: "Test Subject", MAC: station1MAC},
			{Name: "OTHER", MAC: station2MAC},
		},
	}
	it.pubTopic(it.topics.Config(), true, trackingCfg)

	// Start wifi-presence only after subscriptions have been made.
	it.startWifiPresence()

	msg := it.waitMessage(station1ConfigSub)
	t.Logf("station1 config: %s", string(msg.Payload()))
	msg = it.waitMessage(station2ConfigSub)
	t.Logf("station2 config: %s", string(msg.Payload()))

	// Station1 should immediately be set as connected, since hap1
	// is configured to return it as part of the 'Stations' response.
	msg = it.waitMessage(station1StateSub)
	if got, expected := string(msg.Payload()), "connected"; got != expected {
		t.Fatalf("got station1 state %q; expected %q", got, expected)
	}

	// Station2 should immediately be set as not_connected.
	msg = it.waitMessage(station2StateSub)
	if got, expected := string(msg.Payload()), "not_connected"; got != expected {
		t.Fatalf("got station2 state %q; expected %q", got, expected)
	}

	// Invoke 'disconnect' event for station1.
	select {
	case hap1Events <- fmt.Sprintf("AP-STA-DISCONNECTED %s", station1MAC):
		t.Log("Send disconnected", station1MAC)
	case <-time.After(time.Second):
		t.Fatal("timeout sending disconnected event")
	}

	// Verify the 'not_connected' message was published.
	msg = it.waitMessage(station1StateSub)
	t.Log(msg.Topic(), string(msg.Payload()))
	if got, expected := string(msg.Payload()), "not_connected"; got != expected {
		t.Fatalf("unexpected message on topic %s: got %q, expected %q", msg.Topic(), got, expected)
	}

	// Update config to remove station1
	trackingCfg = hass.Configuration{Devices: []hass.TrackConfig{
		{Name: "OTHER", MAC: station2MAC},
	}}
	it.pubTopic(it.topics.Config(), true, trackingCfg)

	msg = it.waitMessage(station1ConfigSub)
	if payload := msg.Payload(); len(payload) != 0 {
		t.Fatalf("station config = %s; expected null", string(payload))
	}
	t.Log("station config: (null)")

	// Invoke 'shutdown' event.
	select {
	case hap1Events <- "CTRL-EVENT-TERMINATING":
		t.Log("Send terminating")
	case <-time.After(time.Second):
		t.Fatal("timeout sending terminating event")
	}

	// Ensure wifi-presence stopped gracefully.
	select {
	case err := <-it.wpErrs:
		if !errors.Is(err, hostapd.ErrTerminating) {
			t.Fatal(err)
		}
		t.Logf("wifi-presence exited with expected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for process to terminate")
	}
}
