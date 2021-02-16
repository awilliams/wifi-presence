package hostapd

import (
	"bytes"
	"net"
	"path"
	"testing"
	"time"
)

func TestNewUnixSocketConn(t *testing.T) {
	// Create Unix socket listener. This will
	// simulate the hostapd's control interface.
	hostapdConn := newMockHostapdConn(t)

	// Create our Conn.

	lpath := path.Join(hostapdConn.tempDir, "local.sock")
	laddr, err := net.ResolveUnixAddr("unixgram", lpath)
	if err != nil {
		t.Fatal(err)
	}

	c, err := newUnixSocketConn(laddr.String(), hostapdConn.path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Send msg over remote socket.

	msg := []byte("hola, ruok?")
	if _, err := hostapdConn.WriteToUnix(msg, laddr); err != nil {
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
	got := buf[:n]

	if !bytes.Equal(got, msg) {
		t.Fatalf("Conn.Read(): got %q; expected %q", string(got), string(msg))
	}
	t.Logf("Conn.rw.Read(): %q", string(got))

	// Send message from conn.

	msg = []byte("imok")
	if err = c.setWriteDeadline(time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(msg); err != nil {
		t.Fatal(err)
	}

	// Confirm message was received.

	n, err = hostapdConn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	got = buf[:n]

	if !bytes.Equal(got, msg) {
		t.Fatalf("Conn.Write(): got %q; expected %q", string(got), string(msg))
	}
	t.Logf("Conn.rw.Write(): %q", string(got))
}
