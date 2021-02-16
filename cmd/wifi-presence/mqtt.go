package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/awilliams/wifi-presence/pkg/presence"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Possible status values.
const (
	statusOnline       = "online"       // Status when program is running
	statusOffline      = "offline"      // Status when program is stopped
	statusDisconnected = "disconnected" // Status when program is unexpectedly disconnected
)

// status is sent on program connect and disconnect to the will topic.
type status struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// mqttClientOptions wraps the mqtt.ClientOptions type with
// additional configurable values.
type mqttClientOptions struct {
	*mqtt.ClientOptions
	apName      string
	topicPrefix string
}

// Set name of the Access Point.
func (m *mqttClientOptions) SetAPName(name string) {
	m.apName = name
}

// Set the MQTT topic prefix. All messages will be prefixed
// with this value.
func (m *mqttClientOptions) SetTopicPrefix(prefix string) {
	m.topicPrefix = prefix
}

type mqttOpt func(*mqttClientOptions)

// newMQTTClient wraps the given MQTT client.
func newMQTTClient(ctx context.Context, opts ...mqttOpt) (*mqttClient, error) {
	mqttOpts := mqttClientOptions{
		ClientOptions: mqtt.NewClientOptions(),
	}
	for _, f := range opts {
		f(&mqttOpts)
	}

	willTopic := path.Join(mqttOpts.topicPrefix, mqttOpts.apName, "status")
	willMsg, err := json.Marshal(status{Status: statusDisconnected, Timestamp: time.Now()})
	if err != nil {
		return nil, err
	}
	mqttOpts.SetWill(willTopic, string(willMsg), 0x2, true)

	c := &mqttClient{
		mqtt:        mqtt.NewClient(mqttOpts.ClientOptions),
		topicPrefix: mqttOpts.topicPrefix,
		willTopic:   willTopic,
	}

	if err := c.waitToken(ctx, c.mqtt.Connect()); err != nil {
		return nil, fmt.Errorf("unable to connect to MQTT broker: %w", err)
	}

	return c, nil
}

// mqttClient wraps an mqtt.Client to provide helper methods.
type mqttClient struct {
	mqtt        mqtt.Client
	topicPrefix string
	willTopic   string
}

// close closes the MQTT connection.
func (c *mqttClient) close() {
	c.mqtt.Disconnect(5000)
}

// publishWill publishes to the will topic a status message.
func (c *mqttClient) publishWill(ctx context.Context, willStatus string) error {
	msg, err := json.Marshal(status{Status: willStatus, Timestamp: time.Now()})
	if err != nil {
		return err
	}

	if err := c.waitToken(ctx, c.mqtt.Publish(c.willTopic, 2, true, msg)); err != nil {
		return fmt.Errorf("unable to publish MQTT will: %w", err)
	}
	return nil
}

// stationEvent publishes a station event.
func (c *mqttClient) stationEvent(ctx context.Context, se presence.StationEvent) error {
	msg, err := json.Marshal(se)
	if err != nil {
		return err
	}

	topic := c.topic(se.AP, se.MAC.String())
	t := c.mqtt.Publish(topic, 2, true, msg)
	return c.waitToken(ctx, t)
}

// topic generates an MQTT topic using the pre-configured prefix
// and provided suffix.
func (c *mqttClient) topic(suffix ...string) string {
	return path.Join(append([]string{c.topicPrefix}, suffix...)...)
}

// waitToken waits for either the context to be cancalled or the token
// operation to complete.
func (c *mqttClient) waitToken(ctx context.Context, token mqtt.Token) error {
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for MQTT: %w", ctx.Err())
	}
}
