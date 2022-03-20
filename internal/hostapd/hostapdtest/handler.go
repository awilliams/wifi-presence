package hostapdtest

import (
	"fmt"
	"strings"
	"sync"
)

// Handler is a collection of user-definable functions
// used for handling HostAPD messages.
type Handler struct {
	sync.Mutex     // Protects following.
	onMessage      func(msg string)
	onUndef        func(msg string) string
	onPing         func() bool // Reply to PING with PONG unless onPing is defined and returns false.
	onStatus       func() StatusResp
	onStationFirst func() (resp StationResp, unknown bool, ok bool) // If unknown is true, then an "UNKNOWN COMMAND" response will be sent.
	onStationNext  func(mac string) (resp StationResp, ok bool)
	onAttach       func() <-chan string
	onDetach       func()
}

// DefaultHostAPDHandler is a convenience function to define a Handler
// that responds with the given status and stations.
func DefaultHostAPDHandler(status StatusResp, stations []StationResp) *Handler {
	var h Handler
	h.OnPing(func() bool { return true })
	h.OnStatus(func() StatusResp { return status })
	h.OnStationFirst(func() (StationResp, bool, bool) {
		if len(stations) > 0 {
			return stations[0], false, true
		}
		return StationResp{}, false, false
	})
	h.OnStationNext(func(mac string) (StationResp, bool) {
		for i, s := range stations {
			if s.MAC == mac {
				if len(stations) > i+1 {
					return stations[i+1], true
				}
				break
			}
		}
		return StationResp{}, false
	})
	return &h
}

// OnMessage registers a callback that will be called
// with every message received after calling Serve.
func (h *Handler) OnMessage(f func(msg string)) {
	h.Lock()
	h.onMessage = f
	h.Unlock()
}

func (h *Handler) handleMessage(msg string) {
	h.Lock()
	defer h.Unlock()
	if h.onMessage == nil {
		return
	}
	h.onMessage(msg)
}

// OnUndef registers a callback that will be called
// with every otherwise unhandled message received after calling Serve.
func (h *Handler) OnUndef(f func(msg string) string) {
	h.Lock()
	h.onUndef = f
	h.Unlock()
}

func (h *Handler) handleUndef(msg string) (string, bool) {
	h.Lock()
	defer h.Unlock()
	if h.onUndef != nil {
		return h.onUndef(msg), true
	}
	return "", false
}

// OnPing registers a callback that will be called
// with every ping message. If false is returned, then
// no PONG reply is sent. If this callback isn't set, then
// PONG will be sent.
func (h *Handler) OnPing(f func() bool) {
	h.Lock()
	h.onPing = f
	h.Unlock()
}

func (h *Handler) handlePing() bool {
	h.Lock()
	defer h.Unlock()
	if h.onPing == nil {
		return true
	}
	return h.onPing()
}

// OnStatus registers a callback which determines the response
// to a STATUS message.
func (h *Handler) OnStatus(f func() StatusResp) {
	h.Lock()
	h.onStatus = f
	h.Unlock()
}

func (h *Handler) handleStatus() (StatusResp, bool) {
	h.Lock()
	defer h.Unlock()
	if h.onStatus == nil {
		return StatusResp{}, false
	}
	return h.onStatus(), true
}

// OnStationFirst registers a callback which determines the response
// to a STA-FIRST message.
func (h *Handler) OnStationFirst(f func() (resp StationResp, unknown, ok bool)) {
	h.Lock()
	h.onStationFirst = f
	h.Unlock()
}

func (h *Handler) handleStationFirst() (StationResp, bool, bool) {
	h.Lock()
	defer h.Unlock()
	if h.onStationFirst == nil {
		return StationResp{}, false, false
	}
	return h.onStationFirst()
}

// OnStationNext registers a callback which determines the response
// to a STA-NEXT message. The callback is passed the mac address received.
func (h *Handler) OnStationNext(f func(mac string) (StationResp, bool)) {
	h.Lock()
	h.onStationNext = f
	h.Unlock()
}

func (h *Handler) handleStationNext(mac string) (StationResp, bool) {
	h.Lock()
	defer h.Unlock()
	if h.onStationNext == nil {
		return StationResp{}, false
	}
	return h.onStationNext(mac)
}

// OnAttach registers a callback function for when ATTACH is received.
// The returned channel will be read from and sent to the remote connection.
// The channel may be closed by the caller.
func (h *Handler) OnAttach(f func() <-chan string) {
	h.Lock()
	h.onAttach = f
	h.Unlock()
}

func (h *Handler) handleAttach() <-chan string {
	h.Lock()
	defer h.Unlock()
	if h.onAttach == nil {
		return nil
	}
	return h.onAttach()
}

// OnDetach registers a callback for when a DETACH message is received.
func (h *Handler) OnDetach(f func()) {
	h.Lock()
	h.onDetach = f
	h.Unlock()
}

func (h *Handler) handleDetach() {
	h.Lock()
	defer h.Unlock()
	if h.onDetach != nil {
		h.onDetach()
	}
}

// StatusResp forms the response to a STATUS message.
type StatusResp struct {
	State      string
	Channel    int
	SSID       string
	BSSID      string
	MaxTxPower int
}

func (s *StatusResp) encode() string {
	var b strings.Builder
	fmt.Fprintf(&b, "state=%s\n", s.State)
	fmt.Fprintf(&b, "channel=%d\n", s.Channel)
	fmt.Fprintf(&b, "max_txpower=%d\n", s.MaxTxPower)
	fmt.Fprintf(&b, "ssid[0]=%s\n", s.SSID)
	fmt.Fprintf(&b, "bssid[0]=%s\n", s.BSSID)
	return b.String()
}

// StationResp forms the response to station related messages.
type StationResp struct {
	MAC    string
	Assoc  bool
	Signal int
}

func (s *StationResp) encode() string {
	var b strings.Builder
	fmt.Fprintln(&b, s.MAC)
	if s.Assoc {
		fmt.Fprintln(&b, "flags=[ASSOC]")
	}
	fmt.Fprintf(&b, "signal=%d", s.Signal)
	return b.String()
}
