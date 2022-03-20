package hass

import (
	"regexp"
	"strings"
)

// MQTTTopics configures MQTT topic generation.
type MQTTTopics struct {
	Name       string
	Prefix     string
	HASSPrefix string // Optional
}

// Will topic for the overall wifi-presence status.
func (m *MQTTTopics) Will() string {
	return mkTopic(m.Prefix, sanitizeTopic(m.Name), "status")
}

// Config topic for configuration changes sent to wifi-presence.
func (m *MQTTTopics) Config() string {
	return mkTopic(m.Prefix, "config")
}

// DeviceDiscovery topic for Home Assistant device tracker configuration.
func (m *MQTTTopics) DeviceDiscovery(mac string) string {
	// https://www.home-assistant.io/docs/mqtt/discovery/#discovery-topic
	// Format: <discovery_prefix>/device_tracker/[<node_id>/]<object_id>/config
	// <node_id> (Optional): ID of the node providing the topic, this is not used by Home Assistant
	//   but may be used to structure the MQTT topic. The ID of the node must
	//   only consist of characters from the character class [a-zA-Z0-9_-] (alphanumerics, underscore and hyphen).
	// <object_id>: The ID of the device. This is only to allow for separate topics for each
	//   device and is not used for the entity_id. The ID of the device must only consist of characters
	//   from the character class [a-zA-Z0-9_-] (alphanumerics, underscore and hyphen).
	return mkTopic(m.HASSPrefix, "device_tracker", sanitizeTopic(m.Name), sanitizeMACTopic(mac), "config")
}

// DeviceState topic for device's state.
func (m *MQTTTopics) DeviceState(mac string) string {
	return mkTopic(m.Prefix, "station", sanitizeTopic(m.Name), sanitizeMACTopic(mac), "state")
}

// DeviceJSONAttrs topic for device's attributes.
func (m *MQTTTopics) DeviceJSONAttrs(mac string) string {
	return mkTopic(m.Prefix, "station", sanitizeTopic(m.Name), sanitizeMACTopic(mac), "attrs")
}

func mkTopic(parts ...string) string {
	return strings.Join(parts, "/")
}

var hassTopicRe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeTopic(v string) string {
	return strings.ToLower(hassTopicRe.ReplaceAllString(v, ""))
}

func sanitizeMACTopic(mac string) string {
	return strings.ToLower(strings.ReplaceAll(mac, ":", "-"))
}
