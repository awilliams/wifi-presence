package hostapd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Hostapd control interface command and response strings.
const (
	cmdStatus       = "STATUS"
	cmdStationFirst = "STA-FIRST"
	cmdStationNext  = "STA-NEXT"
	cmdPing         = "PING"
	respPong        = "PONG"
	cmdAttach       = "ATTACH"
	respAttach      = "OK"
	cmdDetach       = "DETACH"
	respDetach      = "OK"
	unknownCommand  = "UNKNOWN COMMAND"
)

// ErrTerminating is returned by attach when and if the control
// interface terminates the connection. This can be because
// WiFi interface configuration was changed and hostapd is restarting.
var ErrTerminating = errors.New("wpa_supplicant is exiting")

// ErrUnknownCmd is returned when the hostapd socket returns an unknownCommand
// response.
type ErrUnknownCmd string

func (e ErrUnknownCmd) Error() string {
	return fmt.Sprintf("sent command %q, received unknown command response", string(e))
}

// newCtrl returns a new ctrl using the given connection.
func newCtrl(cn *conn, rTimeout, wTimeout time.Duration) (*ctrl, error) {
	c := &ctrl{
		readTimeout:  rTimeout,
		writeTimeout: wTimeout,
		conn:         cn,
		buf:          make([]byte, 2*1024),
	}
	if err := c.ping(); err != nil {
		return nil, fmt.Errorf("ping error: %w", err)
	}

	return c, nil
}

// ctrl manages communication with the hostapd control interface.
type ctrl struct {
	readTimeout, writeTimeout time.Duration

	mu   sync.Mutex // Protects following.
	conn *conn
	buf  []byte
}

// cmd sends the given command and waits for the response. On success, the
// response's data is given to the resp function. Any error returned from
// the resp function is returned by this method. This method is threadsafe.
// The resp function should not retain p.
func (c *ctrl) cmd(cmd string, resp func(p []byte) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		if err := c.conn.setWriteDeadline(c.writeTimeout); err != nil {
			return err
		}
	}
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return err
	}

	if c.readTimeout > 0 {
		if err := c.conn.setReadDeadline(c.readTimeout); err != nil {
			return err
		}
	}

	n, err := c.conn.Read(c.buf)
	if err != nil {
		return fmt.Errorf("read error from %q command: %w", cmd, err)
	}

	if bytes.HasPrefix(c.buf[:n], []byte(unknownCommand)) {
		return ErrUnknownCmd(cmd)
	}

	return resp(c.buf[:n])
}

// ping tests whether the control interface is responding
// to requests.
func (c *ctrl) ping() error {
	return c.cmd(cmdPing, func(resp []byte) error {
		if s := strings.TrimSpace(string(resp)); s != respPong {
			return fmt.Errorf("unexpected response to %s: %q", cmdPing, s)
		}
		return nil
	})
}

// status returns the station's status.
func (c *ctrl) status() (Status, error) {
	var s Status
	return s, c.cmd(cmdStatus, func(resp []byte) error {
		return s.parse(resp)
	})
}

// stationFirst returns the start of the linked list of stations.
// If no station is found, the returned bool is false.
func (c *ctrl) stationFirst() (Station, bool, error) {
	var (
		s  Station
		ok bool
	)
	return s, ok, c.cmd(cmdStationFirst, func(resp []byte) error {
		if len(resp) == 0 {
			return nil
		}
		ok = true
		if err := s.parse(resp); err != nil {
			return fmt.Errorf("command %q error: %w", cmdStationFirst, err)
		}
		return nil
	})
}

// stationNext returns the station following the given mac address
// in the linked list of stations.
// If no station is found, the returned bool is false.
func (c *ctrl) stationNext(mac string) (Station, bool, error) {
	var (
		s  Station
		ok bool
	)
	return s, ok, c.cmd(fmt.Sprintf("%s %s", cmdStationNext, mac), func(resp []byte) error {
		if len(resp) == 0 {
			return nil
		}
		ok = true
		return s.parse(resp)
	})
}

// attach requests that the control interface send unsolicited
// event messages. These include station connection and disconnect events.
// This method blocks until the context is canceled or an error occurs.
// While this method is blocking, no other ctrl methods can be used.
func (c *ctrl) attach(ctx context.Context, cb func(Event) error) error {
	err := c.cmd(cmdAttach, func(resp []byte) error {
		if s := strings.TrimSpace(string(resp)); s != respAttach {
			return fmt.Errorf("unexpected response to %s: %q", cmdAttach, s)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Socket is now "attached". It will receive unsolicited
	// messages, so the socket should no longer be used for other commands
	// until detached. Otherwise command responses and unsolicited messages would
	// be mixed. For this reason, the mutex is held for the duration of the blocking attach method.

	c.mu.Lock()
	defer c.mu.Unlock()

	// Detach when the context is canceled and/or
	// on method exit.

	detach, detached := c.detacher()
	defer detach()
	// Context cancellation stops connection from receiving
	// events by calling detach.
	go func() {
		<-ctx.Done()
		detach()
	}()

	// Remove any read timeouts from the connection, otherwise
	// it could trigger while waiting for events.
	if err = c.conn.unsetReadDeadline(); err != nil {
		return err
	}

	// Read and process unsolicited messages.

	var msg string
	for {
		n, err := c.conn.Read(c.buf)
		if err != nil {
			return err
		}

		msg = strings.TrimSpace(string(c.buf[:n]))

		// Check if this is a DETACH response (OK).
		if msg == respDetach {
			// Now check if it was expected.
			select {
			case <-detached:
				return nil
			default:
				return fmt.Errorf("unexpected message while attached: %q", msg)
			}
		}

		event, err := parseEvent(msg)
		if err != nil {
			return err
		}

		if _, ok := event.(EventTerminating); ok {
			return ErrTerminating
		}

		if err = cb(event); err != nil {
			return err
		}
	}
}

// detacher returns a function that, when called, sends a detach
// command to stop receiving unsolicited events. The function is threadsafe and
// can be called multiple times. Only the first call will perform the detach.
// Once the function has been called, the returned channel will unblock.
// The returned function must be called while under ctrl's conn mutex lock.
func (c *ctrl) detacher() (func(), <-chan struct{}) {
	var (
		mu       sync.Mutex            // Protects following.
		detached bool                  // True after detach command has been sent.
		done     = make(chan struct{}) // Closed after detached is executed.
	)

	f := func() {
		mu.Lock()
		defer mu.Unlock()

		if detached {
			return
		}
		detached = true

		defer close(done)

		// Ignore errors from here till end,
		// since there's no recourse.

		// Send detach request.
		c.conn.setWriteDeadline(c.writeTimeout)
		c.conn.Write([]byte(cmdDetach))

		// Apply read timeout for the expected response.
		// Response will be handled by main read loop.
		c.conn.setReadDeadline(c.readTimeout)
	}

	return f, done
}
