package hostapdtest

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

// NewHostADP creates a mock HostAPD listening on a temporary socket.
func NewHostAPD(sockPath string) (*HostAPD, error) {
	sockAddr, err := net.ResolveUnixAddr("unixgram", sockPath)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUnixgram("unixgram", sockAddr)
	if err != nil {
		return nil, err
	}

	return &HostAPD{
		Addr: sockAddr.String(),
		conn: conn,
		buf:  make([]byte, 128),
	}, nil
}

// HostAPD mocks a HostAPD socket interface.
type HostAPD struct {
	Addr string
	conn *net.UnixConn
	buf  []byte

	mu     sync.Mutex
	closed bool
}

// Close the socket.
func (h *HostAPD) Close() error {
	h.mu.Lock()
	alreadyClosed := h.closed
	h.closed = true
	h.mu.Unlock()
	if alreadyClosed {
		return nil
	}
	return h.conn.Close()
}

// WriteTo writes the message to the given address. Must not
// be used after calling Serve.
func (h *HostAPD) WriteTo(msg string, addr net.Addr) error {
	if _, err := h.conn.WriteTo([]byte(msg), addr); err != nil {
		return fmt.Errorf("WriteTo(%q) err: %w", msg, err)
	}
	return nil
}

// ReadFrom reads a message and returns it as a string along with the
// remote address. Must not be used after calling Serve.
func (h *HostAPD) ReadFrom() (string, net.Addr, error) {
	n, raddr, err := h.conn.ReadFrom(h.buf)
	if err != nil {
		return "", nil, err
	}
	return string(h.buf[:n]), raddr, nil
}

// Serve uses the handler to serve requests. This method
// blocks until an error is encountered.
func (h *HostAPD) Serve(handler *Handler) error {
	done := make(chan struct{})
	defer close(done)

	for {
		msg, raddr, err := h.ReadFrom()
		if err != nil {
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.closed {
				return nil
			}
			return err
		}

		handler.handleMessage(msg)

		switch {
		case msg == "PING":
			if handler.handlePing() {
				if err := h.WriteTo("PONG", raddr); err != nil {
					return err
				}
			}

		case msg == "STATUS":
			if resp, ok := handler.handleStatus(); ok {
				if err := h.WriteTo(resp.encode(), raddr); err != nil {
					return err
				}
			}

		case msg == "STA-FIRST":
			var resp string

			station, unknown, ok := handler.handleStationFirst()
			switch {
			case unknown:
				resp = "UNKNOWN COMMAND"
			case ok:
				resp = station.encode()
			default:
				// Empty response
			}
			if err := h.WriteTo(resp, raddr); err != nil {
				return err
			}

		case strings.HasPrefix(msg, "STA-NEXT"):
			mac := strings.TrimPrefix(msg, "STA-NEXT ")
			if station, ok := handler.onStationNext(mac); ok {
				if err := h.WriteTo(station.encode(), raddr); err != nil {
					return err
				}
			} else {
				if err := h.WriteTo("", raddr); err != nil {
					return err
				}
			}

		case msg == "ATTACH":
			if msgs := handler.handleAttach(); msgs != nil {
				if err := h.WriteTo("OK", raddr); err != nil {
					return err
				}

				go func(msgs <-chan string) {
					for {
						select {
						case <-done:
							return
						case msg := <-msgs:
							if err := h.WriteTo(msg, raddr); err != nil {
								return
							}
						}
					}
				}(msgs)
			}

		case msg == "DETACH":
			// Ignore this error, since the other side of
			// the connection may already have closed.
			_ = h.WriteTo("OK", raddr)
			handler.handleDetach()
			return nil

		default:
			if resp, ok := handler.handleUndef(msg); ok {
				if err := h.WriteTo(resp, raddr); err != nil {
					return err
				}
			}
		}
	}
}
