package hostapd

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"
)

func TestCtrl_cmd(t *testing.T) {
	messages := []struct {
		cmd      string
		expected string
	}{
		{"message #1", "ok"},
		{"message #2", "again?"},
		{"message #3", "bye"},
	}

	hostapdErr := make(chan string, 1)
	var (
		handler hostapdtest.Handler
		pos     int
	)
	handler.OnUndef(func(msg string) string {
		if pos > len(messages) {
			hostapdErr <- "unexpected message"
			return ""
		}
		m := messages[pos]
		pos++

		if msg != m.cmd {
			hostapdErr <- fmt.Sprintf("received %q; want %q", msg, m.cmd)
		}
		return m.expected
	})

	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	go func() {
		if err := hostapd.Serve(&handler); err != nil {
			hostapdErr <- err.Error()
		}
	}()

	c, err := newCtrl(newConn(t, hostapd.Addr), time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range messages {
		err := c.cmd(m.cmd, func(resp []byte) error {
			got := string(resp)
			if got != m.expected {
				t.Errorf("cmd(%q): got response %q; want %q", m.cmd, got, m.expected)
			} else {
				t.Logf("cmd(%q): got response %q", m.cmd, got)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-hostapdErr:
			t.Fatal(err)
		default:
			// pass
		}
	}
}

func TestCtrl_attach(t *testing.T) {
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	var handler hostapdtest.Handler
	hostapdEvents := make(chan string)
	defer close(hostapdEvents)
	detached := make(chan struct{})
	handler.OnAttach(func() <-chan string {
		return hostapdEvents
	})
	handler.OnDetach(func() {
		close(detached)
	})

	// Manage hostapd conn in a separate goroutine.
	hostapdErr := make(chan error, 1)
	go func() {
		if err := hostapd.Serve(&handler); err != nil {
			hostapdErr <- err
		}
	}()

	c, err := newCtrl(newConn(t, hostapd.Addr), time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	attachErr := make(chan error, 1)
	attachEvent := make(chan Event)
	ctx, cancelAttach := context.WithCancel(context.Background())
	defer cancelAttach()
	go func() {
		err := c.attach(ctx, func(e Event) error {
			attachEvent <- e
			return nil
		})
		if err != nil {
			attachErr <- err
		}
	}()

	events := []Event{
		EventStationConnect{fmt.Sprintf("<1>%s %s", eventAPStaConnected, "AB:CD:12:34:56:78"), "AB:CD:12:34:56:78"},
		EventStationDisconnect{fmt.Sprintf("<3>%s %s", eventAPStaDisconnected, "FA:CE:BE:EF:56:78"), "FA:CE:BE:EF:56:78"},
		EventStationConnect{fmt.Sprintf("<3>%s %s", eventAPStaConnected, "12:BA:00:34:56:78"), "12:BA:00:34:56:78"},
		EventStationConnect{fmt.Sprintf("%s %s", eventAPStaConnected, "12:EE:FF:34:56:78"), "12:EE:FF:34:56:78"},
	}

	for _, event := range events {
		// Send events from hostapd conn.
		select {
		case hostapdEvents <- event.Raw():
			// OK
		case <-time.After(time.Second):
			t.Fatal("timeout waiting to send event")
		}

		// Verify receipt of events from ctrl callback.

		select {
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		case err := <-hostapdErr:
			t.Fatalf("hostapd conn error: %v", err)
		case err := <-attachErr:
			t.Fatalf("attach error: %v", err)
		case <-detached:
			t.Fatal("hostapd received unexpected detach message")
		case got := <-attachEvent:
			if got != event {
				t.Errorf("got event %v; expected %v", got, event)
			}
			t.Logf("got event %#v", got)
		}
	}

	// Cancel attach's context. Before returning,
	// it should send a detach message.
	cancelAttach()

	select {
	case err := <-hostapdErr:
		t.Fatalf("error received from hostapd: %v", err)
	case err := <-attachErr:
		t.Fatalf("error received from attach: %v", err)
	case <-detached:
		t.Log("hostapd received detach message")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hostapd to receive detach")
	}
}

func TestCtrl_attach_terminated(t *testing.T) {
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	var handler hostapdtest.Handler
	hostapdEvents := make(chan string)
	detached := make(chan struct{})
	handler.OnAttach(func() <-chan string {
		return hostapdEvents
	})
	handler.OnDetach(func() { close(detached) })

	// Manage hostapd conn in a separate goroutine.
	hostapdErr := make(chan error, 1)
	go func() {
		if err := hostapd.Serve(&handler); err != nil {
			hostapdErr <- err
		}
	}()

	c, err := newCtrl(newConn(t, hostapd.Addr), time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	attachErr := make(chan error, 1)
	ctx, cancelAttach := context.WithCancel(context.Background())
	defer cancelAttach()
	go func() {
		defer close(attachErr)
		err := c.attach(ctx, func(e Event) error {
			return nil
		})
		if err != nil {
			attachErr <- err
		}
	}()

	// Send terminating event from hostapd conn.
	select {
	case hostapdEvents <- eventWPATerminating:
		// OK
	case <-time.After(time.Second):
		t.Fatal("timeout waiting to send event")
	}

	// Verify attach() received an error (terminating).

	select {
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	case err := <-hostapdErr:
		t.Fatalf("hostapd conn error: %v", err)
	case err := <-attachErr:
		if err != ErrTerminating {
			t.Fatalf("attach error %v; expected %v", err, ErrTerminating)
		}
		t.Logf("attach error (expected): %v", err)
	}

	select {
	case <-detached:
		t.Log("hostapd received detach")
	case err := <-hostapdErr:
		t.Fatalf("error received from hostapd: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hostapd to receive detach")
	}
}

// newConn is a helper to create the Unix socket connected
// to the remote address (hostapd).
func newConn(t *testing.T, rAddr string) *conn {
	t.Helper()

	conn, err := newUnixSocketConn(
		path.Join(t.TempDir(), "local.sock"),
		rAddr,
	)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	t.Cleanup(func() { conn.Close() })

	return conn
}
