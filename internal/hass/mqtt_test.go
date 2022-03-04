package hass

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"
)

var mqttAddr = flag.String("mqttAddr", "", "MQTT broker address")

func TestMQTTSubscribeConfig(t *testing.T) {
	expected := []TrackConfig{
		{Name: fmt.Sprintf("test-1-%d", time.Now().UnixNano()), MAC: "FF:00:FF:00:FF:00"},
		{Name: fmt.Sprintf("test-2-%d", time.Now().UnixNano()), MAC: "00:FF:00:FF:00:FF"},
		{Name: fmt.Sprintf("test-3-%d", time.Now().UnixNano()), MAC: "AA:BB:CC:DD:EE:FF"},
	}

	var (
		publishErr   = make(chan error, 1)
		subscribeErr = make(chan error, 1)
		c            = mqttClient(t)
		received     = make(chan Configuration, 1)
		ctx, cancel  = context.WithCancel(context.Background())
	)
	defer cancel()

	go func() {
		b, _ := json.Marshal(Configuration{
			Devices: expected,
		})

		tkn := c.c.Publish(c.topics.Config(), qosExactlyOnce, true, b)
		tkn.WaitTimeout(time.Second)
		publishErr <- tkn.Error()
	}()

	go func() {
		subscribeErr <- c.SubscribeConfig(ctx, func(retained bool, cfg Configuration) error {
			// Ignore messages from previous test runs (if any).
			if !retained {
				received <- cfg
			}
			return nil
		})
	}()

	// Wait first for the Publish to complete.
	select {
	case err := <-publishErr:
		if err != nil {
			t.Fatalf("Publish() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for publish")
	}

	select {
	case err := <-subscribeErr:
		if err != nil {
			t.Fatalf("SubscribeConfig() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting event")
	case got := <-received:
		for i, cfg := range got.Devices {
			t.Logf("got[%d]: %+v", i, cfg)
		}

		if len(got.Devices) != len(expected) {
			t.Fatalf("got %d configurations; want %d", len(got.Devices), len(expected))
		}
		for i, want := range expected {
			if got.Devices[i] != want {
				t.Errorf("got[%d]: %+v; want %+v", i, got.Devices[i], want)
			}
		}
	}
}

func TestMQTTSubscribeConfig_CallbackErr(t *testing.T) {
	var (
		publishErr   = make(chan error, 1)
		subscribeErr = make(chan error, 1)
		expectedErr  = errors.New("oh no!")
		c            = mqttClient(t)
		ctx, cancel  = context.WithCancel(context.Background())
	)
	defer cancel()

	go func() {
		cfgs := Configuration{
			Devices: []TrackConfig{
				{Name: fmt.Sprintf("test-1-%d", time.Now().UnixNano()), MAC: "FF:00:FF:00:FF:00"},
			},
		}
		b, _ := json.Marshal(cfgs)

		tkn := c.c.Publish(c.topics.Config(), qosExactlyOnce, true, b)
		tkn.WaitTimeout(time.Second)
		publishErr <- tkn.Error()
	}()

	go func() {
		subscribeErr <- c.SubscribeConfig(ctx, func(retained bool, _ Configuration) error {
			// Ignore messages from previous test runs (if any).
			if !retained {
				return expectedErr
			}
			return nil
		})
	}()

	// Wait first for the Publish to complete.
	select {
	case err := <-publishErr:
		if err != nil {
			t.Fatalf("Publish() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for publish")
	}

	// Wait for SubscribeConfig to error.
	select {
	case err := <-subscribeErr:
		if err != expectedErr {
			t.Fatalf("SubscribeConfig() error = %v; want %v", err, expectedErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting expected error")
	}
}

func TestMQTTSubscribeConfig_Cancel(t *testing.T) {
	var (
		subscribeErr = make(chan error, 1)
		c            = mqttClient(t)
		ctx, cancel  = context.WithCancel(context.Background())
	)
	defer cancel()

	go func() {
		subscribeErr <- c.SubscribeConfig(ctx, func(_ bool, _ Configuration) error {
			return nil
		})
	}()

	cancel()

	// Wait for SubscribeConfig to return.
	select {
	case err := <-subscribeErr:
		if err != nil {
			t.Fatalf("SubscribeConfig() error = %v; want %v", err, nil)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting event")
	}
}

func mqttClient(t *testing.T) *MQTT {
	t.Helper()
	if *mqttAddr == "" {
		t.Skip("skipping test that requires MQTT broker (set with mqttAddr flag)")
	}

	uid := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()/100)

	opts := MQTTOpts{
		BrokerAddr:      *mqttAddr,
		ClientID:        fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()),
		APName:          uid,
		TopicPrefix:     uid,
		DiscoveryPrefix: "hass-" + uid,
	}
	t.Logf("ClientID: %s", opts.ClientID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	c, err := NewMQTT(ctx, opts)
	if err != nil {
		t.Fatalf("NewMQTT() err: %v", err)
	}
	t.Cleanup(c.Close)

	return c
}
