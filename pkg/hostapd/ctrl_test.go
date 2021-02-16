package hostapd

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"
)

func TestCtrl_cmd(t *testing.T) {
	hostapd := newMockHostapdConn(t)

	conn, err := newUnixSocketConn(
		path.Join(hostapd.tempDir, "local.sock"),
		hostapd.path,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	const (
		testReq  = "This is a test request"
		testResp = "Test response"
	)

	// Manage hostapd conn in a separate goroutine.
	hostapdErr := make(chan error, 1)
	go func() {
		if _, err := hostapd.pong(); err != nil {
			hostapdErr <- err
			return
		}

		if err := hostapd.setReadDeadline(2 * time.Second); err != nil {
			hostapdErr <- err
			return
		}
		buf := make([]byte, 64)
		n, err := hostapd.Read(buf)
		if err != nil {
			hostapdErr <- err
			return
		}

		got := string(buf[:n])
		if got != testReq {
			hostapdErr <- fmt.Errorf("hostapd conn got %q; expected %q", got, testReq)
			return
		}
		t.Logf("hostapd conn got %q", got)

		if _, err := hostapd.WriteTo([]byte(testResp), conn.LocalAddr()); err != nil {
			hostapdErr <- err
			return
		}
	}()

	c, err := newCtrl(conn, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = c.cmd(testReq, func(resp []byte) error {
		got := string(resp)
		if got != testResp {
			t.Errorf("got response %q; expected %q", got, testResp)
		}
		t.Logf("got response %q", got)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case err = <-hostapdErr:
		t.Fatal(err)
	default:
		// pass
	}
}

func TestCtrl_attach(t *testing.T) {
	hostapd := newMockHostapdConn(t)

	conn, err := newUnixSocketConn(
		path.Join(hostapd.tempDir, "local.sock"),
		hostapd.path,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Manage hostapd conn in a separate goroutine.
	hostapdErr := make(chan error, 1)
	hostapdEvents := make(chan string)
	go func() {
		if err := hostapd.handleAttach(t, hostapdEvents); err != nil {
			hostapdErr <- err
		}
	}()

	c, err := newCtrl(conn, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	attachErr := make(chan error, 1)
	attachEvent := make(chan Event)
	ctx, cancelAttach := context.WithCancel(context.Background())
	defer cancelAttach()
	go func() {
		defer close(attachErr)

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
		case got := <-attachEvent:
			if got != event {
				t.Errorf("got event %v; expected %v", got, event)
			}
			t.Logf("got event %#v", got)
		}
	}

	// Trigger hostapd to wait for detach message.
	close(hostapdEvents)
	// Cancel attach's context. Before returning, it should send a detach message.
	// The attachErr channel will be closed after attach() returns.
	cancelAttach()

	select {
	case err := <-hostapdErr:
		t.Fatalf("error received from hostapd: %v", err)
	case err, ok := <-attachErr:
		if ok {
			t.Fatalf("error received from attach: %v", err)
		}
		t.Log("attach returned with no error")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hostapd to receive detach")
	}
}

func TestCtrl_attach_terminated(t *testing.T) {
	hostapd := newMockHostapdConn(t)

	conn, err := newUnixSocketConn(
		path.Join(hostapd.tempDir, "local.sock"),
		hostapd.path,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Manage hostapd conn in a separate goroutine.
	hostapdErr := make(chan error, 1)
	hostapdEvents := make(chan string)
	go func() {
		defer close(hostapdErr)
		if err := hostapd.handleAttach(t, hostapdEvents); err != nil {
			hostapdErr <- err
		}
	}()

	c, err := newCtrl(conn, time.Second, time.Second)
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

	// Trigger hostapd to wait for detach message.
	close(hostapdEvents)

	select {
	case err, ok := <-hostapdErr:
		if ok {
			t.Fatalf("error received from hostapd: %v", err)
		}
		t.Log("hostapd conn received detach")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hostapd to receive detach")
	}
}
