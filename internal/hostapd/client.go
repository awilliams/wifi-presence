package hostapd

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"time"
)

// NewClient connects to the hostap control interface located
// at ctrlSock.
func NewClient(localSockDir, ctrlSock string) (*Client, error) {
	if localSockDir == "" {
		localSockDir = os.TempDir()
	}
	lpath := path.Join(
		localSockDir,
		fmt.Sprintf("wp.%s", path.Base(ctrlSock)),
	)
	if err := isValidSocketPath(lpath); err != nil {
		return nil, err
	}

	conn, err := newUnixSocketConn(lpath, ctrlSock)
	if err != nil {
		return nil, err
	}

	ctrl, err := newCtrl(conn, time.Second, time.Second)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Client{
		localSockDir: localSockDir,
		ctrlSock:     ctrlSock,
		conn:         conn,
		ctrl:         ctrl,
	}, nil
}

// Client is a hostapd control interface client.
type Client struct {
	localSockDir string
	ctrlSock     string
	conn         *conn
	ctrl         *ctrl
}

// Close closes the connection to the control interface. The client
// is no longer usable after closing.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Status returns the station's status.
func (c *Client) Status() (Status, error) {
	return c.ctrl.status()
}

// Stations returns the connected stations.
// Note that stations with Associated=false should
// not be considered as connected. This state can happen if
// Stations is called immediately after a station disconnects.
func (c *Client) Stations() ([]Station, error) {
	var stations []Station

	station, ok, err := c.ctrl.stationFirst()
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	stations = append(stations, station)

	for {
		station, ok, err = c.ctrl.stationNext(station.MAC)
		if !ok {
			break
		}
		if err != nil {
			return nil, err
		}
		stations = append(stations, station)
	}

	return stations, nil
}

// Attach subscribes to hostapd events. For each event, the provided
// callback function will be called. The callback should return quickly, since
// it blocks attach from processing. Attach blocks until an error occurs
// or the provided context is canceled. Any error returned from the provided
// callback will stop attach and be returned.
func (c *Client) Attach(ctx context.Context, events func(Event) error) error {
	// Create a separate socket local to this method. This allows
	// the Client's main socket to still be used while attach is in use.

	lpath := path.Join(
		c.localSockDir,
		fmt.Sprintf("wp-attach.%s", path.Base(c.ctrlSock)),
	)
	if err := isValidSocketPath(lpath); err != nil {
		return err
	}

	conn, err := newUnixSocketConn(lpath, c.ctrlSock)
	if err != nil {
		return fmt.Errorf("unable to create 'attach' socket: %w", err)
	}
	defer conn.Close()

	ctrl, err := newCtrl(conn, time.Second, time.Second)
	if err != nil {
		return err
	}

	return ctrl.attach(ctx, events)
}

// isValidSocketPath returns an error if the given path is invalid for a
// Unix socket.
func isValidSocketPath(p string) error {
	// https://github.com/golang/go/issues/6895
	if runtime.GOOS == "darwin" && len(p) > 104 {
		return fmt.Errorf("socket path (%q) too long", p)
	}
	return nil
}
