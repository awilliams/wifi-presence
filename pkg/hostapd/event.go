package hostapd

import (
	"errors"
	"fmt"
	"strings"
)

// Strings used by hostapd control interface. Source for these, and
// others, is in the wpa_supplicant / hostapd documentation and source
// code: https://w1.fi/wpa_supplicant/devel/ctrl_iface_page.html.
const (
	eventAPStaDisconnected = "AP-STA-DISCONNECTED"
	eventAPStaConnected    = "AP-STA-CONNECTED"
	eventWPATerminating    = "CTRL-EVENT-TERMINATING"
)

// Event is an unsolicited message received from the hostapd control interface.
type Event interface {
	// Raw returns the string used by the control interface
	// to represent this event.
	Raw() string
}

// parseEvent parses the received msg into an Event.
func parseEvent(msg string) (Event, error) {
	if len(msg) == 0 {
		return nil, errors.New("empty message")
	}

	raw := msg

	// Events may be prefixed with a priority level, e.g. '<3>'.
	// Strip this prefix if present.
	if msg[0] == '<' && len(msg) >= 3 && msg[2] == '>' {
		msg = msg[3:]
	}

	switch {
	case strings.HasPrefix(msg, eventAPStaConnected):
		// Station connect event. Example:
		// "<3>AP-STA-CONNECTED 04:ab:00:12:34:56"

		mac := strings.TrimSpace(strings.TrimPrefix(msg, eventAPStaConnected))
		if !isMAC(mac) {
			return nil, fmt.Errorf("invalid MAC address %q", mac)
		}
		return EventStationConnect{raw: raw, MAC: mac}, nil

	case strings.HasPrefix(msg, eventAPStaDisconnected):
		// Station disconnect event. Example:
		// "<3>AP-STA-DISCONNECTED 04:ab:00:12:34:56"

		mac := strings.TrimSpace(strings.TrimPrefix(msg, eventAPStaDisconnected))
		if !isMAC(mac) {
			return nil, fmt.Errorf("invalid MAC address %q", mac)
		}
		return EventStationDisconnect{raw: raw, MAC: mac}, nil

	case msg == eventWPATerminating:
		return EventTerminating(raw), nil

	default:
		return EventUnrecognized(raw), nil
	}
}

// EventStationConnect is an event that happens when a
// station (WiFi client) connects to the AP.
type EventStationConnect struct {
	raw string
	MAC string
}

// Raw returns event as given by hostapd. Satisifes
// the Event interface.
func (e EventStationConnect) Raw() string {
	return e.raw
}

// EventStationDisconnect is an event that happens when a
// station (WiFi client) disconnects from the AP.
type EventStationDisconnect struct {
	raw string
	MAC string
}

// Raw returns event as given by hostapd. Satisifes
// the Event interface.
func (e EventStationDisconnect) Raw() string {
	return e.raw
}

// EventTerminating is received when the wpa_supplicant is exiting.
// This can happen, for example, when the wireless settings are changed
// and hostapd is restarted.
type EventTerminating string

// Raw returns event as given by hostapd. Satisifes
// the Event interface.
func (e EventTerminating) Raw() string {
	return string(e)
}

// EventUnrecognized is a catch-all event for unrecognized
// events. Its Raw method returns the contents of the message.
type EventUnrecognized string

// Raw returns event as given by hostapd. Satisifes
// the Event interface.
func (e EventUnrecognized) Raw() string {
	return string(e)
}
