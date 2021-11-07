package hostapd

import (
	"net"
	"os"
	"time"
)

// newUnixSocketConn creates a connection with a Unix domain socket at
// remotePath. The localPath is used for the local Unix socket file and
// is typically in a temporary directory.
func newUnixSocketConn(localPath, remotePath string) (*conn, error) {
	laddr, err := net.ResolveUnixAddr("unixgram", localPath)
	if err != nil {
		return nil, err
	}

	raddr, err := net.ResolveUnixAddr("unixgram", remotePath)
	if err != nil {
		return nil, err
	}

	c, err := net.DialUnix("unixgram", laddr, raddr)
	if err != nil {
		return nil, err
	}

	return &conn{
		localSock: laddr.String(),
		UnixConn:  *c,
	}, nil
}

// conn is a connection to hostapd's control interface.
type conn struct {
	localSock string
	net.UnixConn
}

func (c *conn) setReadDeadline(timeout time.Duration) error {
	return c.SetReadDeadline(time.Now().Add(timeout))
}

func (c *conn) unsetReadDeadline() error {
	return c.SetReadDeadline(time.Time{})
}

func (c *conn) setWriteDeadline(timeout time.Duration) error {
	return c.SetWriteDeadline(time.Now().Add(timeout))
}

// Close closes the connection and deletes the local
// socket file.
func (c *conn) Close() error {
	cErr := c.UnixConn.Close()
	// Remove local socket file.
	fErr := os.Remove(c.localSock)

	if cErr != nil {
		return cErr
	}
	return fErr
}
