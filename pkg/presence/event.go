package presence

import "time"

// Possible StationEvent actions.
const (
	ActionConnect    = "connect"
	ActionDisconnect = "disconnect"
)

// StationEvent represents an action by a WiFi station (client).
// These events are serialized into JSON and published to MQTT by wifi-presence.
type StationEvent struct {
	AP        string    `json:"ap"`
	SSID      string    `json:"ssid"`
	BSSID     string    `json:"bssid"`
	MAC       MAC       `json:"mac"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
}

// IsConnect returns true if the event's action is connect.
func (s *StationEvent) IsConnect() bool { return s.Action == ActionConnect }

// IsDisconnect returns true if the event's action is disconnect.
func (s *StationEvent) IsDisconnect() bool { return s.Action == ActionDisconnect }
