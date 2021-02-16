package hostapd

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"
)

// NewClient connects to the hostap control interface located
// at ctrlSock.
func NewClient(ctrlSock string) (*Client, error) {
	lpath := path.Join(
		os.TempDir(),
		fmt.Sprintf("wifi-presence.%s.sock", path.Base(ctrlSock)),
	)

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
		ctrlSock: ctrlSock,
		conn:     conn,
		ctrl:     ctrl,
	}, nil
}

// Client is a hostapd control interface client.
type Client struct {
	ctrlSock string
	conn     *conn
	ctrl     *ctrl
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
		os.TempDir(),
		fmt.Sprintf("wifi-presence.%s.attach.sock", path.Base(c.ctrlSock)),
	)

	conn, err := newUnixSocketConn(lpath, c.ctrlSock)
	if err != nil {
		return err
	}

	ctrl, err := newCtrl(conn, time.Second, time.Second)
	if err != nil {
		conn.Close()
		return err
	}
	defer conn.Close()

	return ctrl.attach(ctx, events)
}
