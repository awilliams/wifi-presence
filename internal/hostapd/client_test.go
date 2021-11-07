package hostapd

import (
	"context"
	"errors"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"
)

func TestClient_Status(t *testing.T) {
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	statusResp := hostapdtest.StatusResp{
		State:      "OK",
		Channel:    42,
		SSID:       "test-ssid",
		BSSID:      "test-bssid",
		MaxTxPower: 1122,
	}
	handler := hostapdtest.DefaultHostAPDHandler(statusResp, nil)
	go hostapd.Serve(handler)

	client, err := NewClient(t.TempDir(), hostapd.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	got, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("got Status: %+v", got)

	if got.State != statusResp.State {
		t.Errorf("got State %q; want %q", got.State, statusResp.State)
	}
	if got.Channel != statusResp.Channel {
		t.Errorf("got Channel %d; want %d", got.Channel, statusResp.Channel)
	}
	if got.SSID != statusResp.SSID {
		t.Errorf("got SSID %q; want %q", got.SSID, statusResp.SSID)
	}
	if got.BSSID != statusResp.BSSID {
		t.Errorf("got BSSID %q; want %q", got.BSSID, statusResp.BSSID)
	}
	if got.MaxTxPower != statusResp.MaxTxPower {
		t.Errorf("got MaxTxPower %d; want %d", got.MaxTxPower, statusResp.MaxTxPower)
	}
}

func TestClient_Stations(t *testing.T) {
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	stations := []hostapdtest.StationResp{
		{
			MAC:    "FF:FF:FF:00:00:01",
			Assoc:  true,
			Signal: 1,
		},
		{
			MAC:    "FF:FF:00:00:00:02",
			Assoc:  false,
			Signal: 2,
		},
		{
			MAC:    "FF:00:00:00:00:03",
			Assoc:  true,
			Signal: 3,
		},
	}

	handler := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{}, stations)
	go hostapd.Serve(handler)

	client, err := NewClient(t.TempDir(), hostapd.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	got, err := client.Stations()
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(stations) {
		t.Fatalf("got %d stations; want %d", len(got), len(stations))
	}

	for i, expected := range stations {
		t.Logf("got Stations[%d]: %+v", i, got[i])
		if got[i].MAC != expected.MAC {
			t.Errorf("got Stations[%d].MAC %q; want %q", i, got[i].MAC, expected.MAC)
		}
		if got[i].Associated != expected.Assoc {
			t.Errorf("got Stations[%d].Associated %v; want %v", i, got[i].Associated, expected.Assoc)
		}
		if got[i].Signal != expected.Signal {
			t.Errorf("got Stations[%d].Associated %d; want %d", i, got[i].Signal, expected.Signal)
		}
	}
}

func TestClient_Attach(t *testing.T) {
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	handler := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{}, nil)
	hostapdAttach := make(chan string)
	handler.OnAttach(func() <-chan string {
		return hostapdAttach
	})
	go hostapd.Serve(handler)

	client, err := NewClient(t.TempDir(), hostapd.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, attachCancel := context.WithCancel(context.Background())
	defer attachCancel()
	attachEvents := make(chan Event)
	attachErr := make(chan error, 1)
	go func() {
		defer close(attachErr)
		err := client.Attach(ctx, func(event Event) error {
			attachEvents <- event
			return nil
		})
		if err != nil {
			attachErr <- err
		}
	}()

	events := []Event{
		EventStationConnect{raw: fmt.Sprintf("%s 00:00:00:00:00:FF", eventAPStaConnected)},
		EventStationDisconnect{raw: fmt.Sprintf("%s 00:FF:AA:CC:DD:11", eventAPStaDisconnected)},
		EventUnrecognized("not-a-real-event"),
		EventStationConnect{raw: fmt.Sprintf("%s FF:FF:FF:FF:FF:FF", eventAPStaConnected)},
	}

	for _, event := range events {
		select {
		case hostapdAttach <- event.Raw():
			// OK
		case err := <-attachErr:
			t.Fatalf("Attach() error: %v", err)
		case <-time.After(time.Second):
			t.Fatal("timeout sending to hostapd events chan")
		}

		select {
		case got := <-attachEvents:
			t.Logf("received event: %q", got)
		case err := <-attachErr:
			t.Fatalf("Attach() error: %v", err)
		case <-time.After(time.Second):
			t.Fatal("timeout reading from events chan")
		}
	}

	attachCancel()

	// Allow Attach time to finish.
	select {
	case <-attachErr:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for attach to finish")
	}
}

func TestClient_Attach_term(t *testing.T) {
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	handler := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{}, nil)
	hostapdAttach := make(chan string)
	handler.OnAttach(func() <-chan string {
		return hostapdAttach
	})
	go hostapd.Serve(handler)

	client, err := NewClient(t.TempDir(), hostapd.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, attachCancel := context.WithCancel(context.Background())
	defer attachCancel()
	attachEvents := make(chan Event)
	attachErr := make(chan error, 1)
	go func() {
		err := client.Attach(ctx, func(event Event) error {
			attachEvents <- event
			return nil
		})
		if err != nil {
			attachErr <- err
		}
	}()

	terminating := EventTerminating(eventWPATerminating)
	select {
	case hostapdAttach <- terminating.Raw():
		// OK
	case err := <-attachErr:
		t.Fatalf("Attach() error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timeout sending to hostapd events chan")
	}

	select {
	case got := <-attachEvents:
		t.Fatalf("unexpected received event: %q", got)
	case err := <-attachErr:
		if !errors.Is(err, ErrTerminating) {
			t.Fatalf("Attach() error %q; want %q", err, ErrTerminating)
		}
		t.Logf("Attach() error (expected) %q", err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for attach error")
	}
}
