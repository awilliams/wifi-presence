package hass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	PayloadHome    = "connected"
	PayloadNotHome = "not_connected"
	SourceRouter   = "router"

	icon = "mdi:wifi-marker" // https://materialdesignicons.com/icon/wifi-marker
)

// MQTT QoS Values.
const (
	qosAtMostOnce  = 0x00
	qosAtLeastOnce = 0x01
	qosExactlyOnce = 0x02
)

// MQTTOpts configured an MQTT instance.
type MQTTOpts struct {
	BrokerAddr         string // Required
	ClientID           string // Required
	Username, Password string // Optional

	APName          string // Required
	TopicPrefix     string // Optional
	DiscoveryPrefix string // Optional
}

func NewMQTT(ctx context.Context, opts MQTTOpts) (*MQTT, error) {
	if opts.APName == "" {
		return nil, errors.New("APName cannot be blank")
	}

	if opts.ClientID == "" {
		opts.ClientID = "wifi-presence:" + opts.APName
	}

	topics := MQTTTopics{
		Name:       opts.APName,
		Prefix:     opts.TopicPrefix,
		HASSPrefix: opts.DiscoveryPrefix,
	}
	if topics.Prefix == "" {
		topics.Prefix = "wifi-presence"
	}
	if topics.HASSPrefix == "" {
		topics.HASSPrefix = "homeassistant"
	}

	connLostErrs := make(chan error, 1)

	o := mqtt.NewClientOptions()
	o.AddBroker(opts.BrokerAddr)
	o.SetClientID(opts.ClientID)
	o.SetCleanSession(false)
	o.SetConnectRetry(false)  // Abort only.
	o.SetAutoReconnect(false) // Abort only.
	o.SetKeepAlive(2 * time.Minute)
	if opts.Username != "" || opts.Password != "" {
		o.SetCredentialsProvider(mqtt.CredentialsProvider(func() (username string, password string) {
			return opts.Username, opts.Password
		}))
	}
	o.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		err = fmt.Errorf("MQTT connection lost: %w", err)
		select {
		case connLostErrs <- err:
		default:
		}
	})

	o.SetWill(topics.Will(), StatusOffline, qosAtLeastOnce, true)

	c := mqtt.NewClient(o)
	tkn := c.Connect()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for MQTT Connect: %w", ctx.Err())
	case <-tkn.Done():
		if err := tkn.Error(); err != nil {
			return nil, fmt.Errorf("MQTT Connect error: %w", err)
		}
	}

	return &MQTT{
		c:            c,
		connLostErrs: connLostErrs,
		topics:       &topics,
		apName:       opts.APName,
	}, nil
}

type MQTT struct {
	c            mqtt.Client
	connLostErrs <-chan error

	topics *MQTTTopics
	apName string
}

func (m *MQTT) OnConnectionLost(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-m.connLostErrs:
		return err
	}
}

// Close the MQTT connection.
func (m *MQTT) Close() {
	m.c.Disconnect(2500)
}

// StatusOnline publishes that wifi-presence is online using
// the same topic as the will.
func (m *MQTT) StatusOnline(ctx context.Context) error {
	return m.publishStatus(ctx, StatusOnline)
}

// StatusOnline publishes that wifi-presence is offline using
// the same topic as the will.
func (m *MQTT) StatusOffline(ctx context.Context) error {
	return m.publishStatus(ctx, StatusOffline)
}

func (m *MQTT) publishStatus(ctx context.Context, status string) error {
	tkn := m.c.Publish(m.topics.Will(), qosExactlyOnce, true, status)
	return tokenWait(ctx, tkn, "publish station status")
}

// Discovery is used to publish Home Assistant MQTT discovery configuration.
type Discovery struct {
	Name string
	MAC  string
}

var hassObjectIDRe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// RegisterDeviceTracker publishes a message for Home Assistant to start tracking
// the defined device.
func (m *MQTT) RegisterDeviceTracker(ctx context.Context, dsc Discovery) error {
	if dsc.Name == "" {
		return errors.New("invalid Discovery; Name cannot be blank")
	}
	if dsc.MAC == "" {
		return errors.New("invalid Discovery; MAC cannot be blank")
	}

	// HomeAssistant will replace ':' with '_', so here we proactively
	// create more friendly versions of the MAC and AP name.
	deviceID := strings.ToLower(
		strings.ReplaceAll(dsc.MAC, ":", "") + "_" + hassObjectIDRe.ReplaceAllString(m.apName, ""),
	)

	dt := DeviceTracker{
		AvailabilityTopic: m.topics.Will(),
		Device: Device{
			Name:         dsc.Name,
			Connections:  [][2]string{{"mac", dsc.MAC}},
			Manufacturer: VendorByMAC(dsc.MAC),
			ViaDevice:    m.apName,
		},
		Icon:                icon,
		JSONAttributesTopic: m.topics.DeviceJSONAttrs(dsc.MAC),
		Name:                fmt.Sprintf("%s %s", dsc.Name, m.apName),
		ObjectID:            deviceID,
		PayloadAvailable:    StatusOnline,
		PayloadNotAvailable: StatusOffline,
		PayloadHome:         PayloadHome,
		PayloadNotHome:      PayloadNotHome,
		QOS:                 qosExactlyOnce,
		SourceType:          SourceRouter,
		StateTopic:          m.topics.DeviceState(dsc.MAC),
		UniqueID:            fmt.Sprintf("wifipresence_%s", deviceID),
	}
	payload, err := json.Marshal(dt)
	if err != nil {
		return err
	}

	tkn := m.c.Publish(m.topics.DeviceDiscovery(dsc.MAC), qosExactlyOnce, true, payload)
	return tokenWait(ctx, tkn, "publish station discovery")
}

// RegisterDeviceTracker publishes a message for Home Assistant to stop tracking
// the defined device.
func (m *MQTT) UnregisterDeviceTracker(ctx context.Context, mac string) error {
	if mac == "" {
		return errors.New("MAC cannot be blank")
	}

	tkn := m.c.Publish(m.topics.DeviceDiscovery(mac), qosExactlyOnce, true, []byte{})
	return tokenWait(ctx, tkn, "publish station un-discovery")
}

// StationHome publishes the device's state as 'home'.
func (m *MQTT) StationHome(ctx context.Context, mac string) error {
	return m.publishStationState(ctx, mac, PayloadHome)
}

// StationHome publishes the device's state as 'not_home'.
func (m *MQTT) StationNotHome(ctx context.Context, mac string) error {
	return m.publishStationState(ctx, mac, PayloadNotHome)
}

func (m *MQTT) publishStationState(ctx context.Context, mac, state string) error {
	tkn := m.c.Publish(m.topics.DeviceState(mac), qosExactlyOnce, true, state)
	return tokenWait(ctx, tkn, "publish station state")
}

// StationHome publishes the device's attributes.
func (m *MQTT) StationAttributes(ctx context.Context, mac string, attrs Attrs) error {
	payload, err := json.Marshal(attrs)
	if err != nil {
		return err
	}

	tkn := m.c.Publish(m.topics.DeviceJSONAttrs(mac), qosAtLeastOnce, true, payload)
	return tokenWait(ctx, tkn, "publish station attrs")
}

// ConfigTopic is the MQTT topic that SubscribeConfig will listen to.
func (m *MQTT) ConfigTopic() string {
	return m.topics.Config()
}

// SubscribeConfig registers the callback to receive configuration messages.
// The method blocks until either the provided context is cancelled, an error occurs,
// or the callback function returns a non-nil error.
func (m *MQTT) SubscribeConfig(ctx context.Context, cb func(retained bool, cfg Configuration) error) error {
	errs := make(chan error, 1)
	onError := func(err error) {
		select {
		case errs <- err:
		case <-ctx.Done():
		}
	}

	topic := m.topics.Config()
	tkn := m.c.Subscribe(topic, qosExactlyOnce, func(_ mqtt.Client, msg mqtt.Message) {
		defer msg.Ack()

		var cfg Configuration
		if err := json.Unmarshal(msg.Payload(), &cfg); err != nil {
			onError(fmt.Errorf("unable to decode %q message: %w", msg.Topic(), err))
			return
		}

		if err := cb(msg.Retained(), cfg); err != nil {
			onError(err)
		}
	})

	if err := tokenWait(ctx, tkn, "subscribe tracking config"); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return tokenWait(ctx, m.c.Unsubscribe(topic), "unsubscribe tracking config")

	case err := <-errs:
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tokenWait(ctx, m.c.Unsubscribe(topic), "") // Best effort attempt at unsubscribing.
		return err
	}
}

// tokenWait waits for an MQTT token to complete, otherwise returning an error.
func tokenWait(ctx context.Context, tkn mqtt.Token, description string) error {
	select {
	case <-tkn.Done():
		if err := tkn.Error(); err != nil {
			return fmt.Errorf("mqtt token error (%s): %w", description, err)
		}
	case <-time.After(time.Second):
		return fmt.Errorf("mqtt timeout waiting for token completion (%s): %w", description, ctx.Err())
	}
	return nil
}
