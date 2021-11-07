package hostapd

import (
	"net"
	"path"
	"testing"
	"time"

	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"
)

func TestNewUnixSocketConn(t *testing.T) {
	// Create Unix socket listener. This will
	// simulate the hostapd's control interface.
	hostapd, err := hostapdtest.NewHostAPD(path.Join(t.TempDir(), "hap"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hostapd.Close() })

	// Create our Conn.

	lpath := path.Join(t.TempDir(), "local.sock")
	laddr, err := net.ResolveUnixAddr("unixgram", lpath)
	if err != nil {
		t.Fatal(err)
	}

	c, err := newUnixSocketConn(laddr.String(), hostapd.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Send msg over remote socket.

	msg := "hola, ruok?"
	if err := hostapd.WriteTo(msg, laddr); err != nil {
		t.Fatal(err)
	}

	// Confirm msg was received by conn.

	buf := make([]byte, 128)
	if err = c.setReadDeadline(time.Second); err != nil {
		t.Fatal(err)
	}
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	got := string(buf[:n])

	if got != msg {
		t.Fatalf("Conn.Read(): got %q; expected %q", string(got), string(msg))
	}
	t.Logf("Conn.rw.Read(): %q", string(got))

	// Send message from conn.

	resp := []byte("imok")
	if err = c.setWriteDeadline(time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(resp); err != nil {
		t.Fatal(err)
	}

	// Confirm message was received.

	msg, _, err = hostapd.ReadFrom()
	if err != nil {
		t.Fatal(err)
	}

	if msg != string(resp) {
		t.Fatalf("Conn.Write(): got %q; expected %q", msg, string(resp))
	}
	t.Logf("Conn.rw.Write(): %q", string(got))
}
