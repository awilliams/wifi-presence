package hostapd

import (
	"fmt"
	"net"
	"os"
	"path"
	"testing"
	"time"
)

// newMockHostapdConn creates a listening Unix socket that
// can be used to simulate a hostapd control interface socket.
func newMockHostapdConn(t *testing.T) mockHostapdConn {
	// Create Unix socket listener. This will
	// simulate the hostapd's control interface.

	t.Helper()

	// Using os.TempDir() instead of t.TempDir() since
	// the latter can create long directory paths that
	// cause issues with Unix socket creation.
	tempDir := path.Join(os.TempDir(), t.Name())
	if err := os.MkdirAll(tempDir, 0777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	rpath := path.Join(tempDir, "remote.sock")
	raddr, err := net.ResolveUnixAddr("unixgram", rpath)
	if err != nil {
		t.Fatal(err)
	}

	lc, err := net.ListenUnixgram("unixgram", raddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lc.Close() })

	return mockHostapdConn{
		tempDir:  tempDir,
		path:     raddr.String(),
		UnixConn: lc,
	}
}

type mockHostapdConn struct {
	tempDir string
	path    string
	*net.UnixConn
}

// handleAttach waits for a PING and ATTACH command, then
// sends events received on the events channel to the remote connection
// until the channel is closed. Once the channel is closed, the method
// waits for a DETACH command.
func (m *mockHostapdConn) handleAttach(t *testing.T, events <-chan string) error {
	if _, err := m.pong(); err != nil {
		return err
	}

	if err := m.setReadDeadline(2 * time.Second); err != nil {
		return err
	}
	buf := make([]byte, 64)
	n, raddr, err := m.ReadFromUnix(buf)
	if err != nil {
		return err
	}

	got := string(buf[:n])
	if got != cmdAttach {
		return fmt.Errorf("hostapd conn got %q; expected %q", got, cmdAttach)
	}
	t.Logf("hostapd conn got %q", got)

	if _, err := m.WriteTo([]byte(respAttach), raddr); err != nil {
		return err
	}
	t.Logf("hostapd wrote %q", respAttach)

	for event := range events {
		if _, err := m.WriteTo([]byte(event), raddr); err != nil {
			return err
		}
		t.Logf("hostapd wrote %q", event)
	}

	n, _, err = m.ReadFromUnix(buf)
	if err != nil {
		return err
	}
	got = string(buf[:n])
	if got != cmdDetach {
		return fmt.Errorf("hostapd conn got %q; expected %q", got, cmdDetach)
	}
	t.Logf("hostapd conn got %q", got)

	if _, err := m.WriteTo([]byte(respDetach), raddr); err != nil {
		return err
	}
	t.Logf("hostapd wrote %q", respDetach)

	return nil
}

func (m *mockHostapdConn) setReadDeadline(timeout time.Duration) error {
	return m.SetReadDeadline(time.Now().Add(timeout))
}

// pong waits for a PING command and responds with a PONG.
func (m *mockHostapdConn) pong() (*net.UnixAddr, error) {
	if err := m.setReadDeadline(2 * time.Second); err != nil {
		return nil, err
	}

	buf := make([]byte, 64)
	n, raddr, err := m.ReadFromUnix(buf)
	if err != nil {
		return nil, err
	}

	got := string(buf[:n])
	if got != cmdPing {
		return nil, fmt.Errorf("got %q; expected %q", got, cmdPing)
	}

	_, err = m.WriteToUnix([]byte(respPong), raddr)
	return raddr, err
}
