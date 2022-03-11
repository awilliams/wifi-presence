package presence

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/awilliams/wifi-presence/internal/hass"
	"github.com/awilliams/wifi-presence/internal/hostapd"

	"golang.org/x/sync/errgroup"
)

// Opt is a configuration option for Daemon.
type Opt func(*Daemon)

// WithAPName sets the name of the Access Point. This name is
// used for station attributes and topic path creation.
func WithAPName(name string) Opt {
	return func(d *Daemon) {
		d.apName = name
	}
}

// WithHassOpt is required and sets the hass.MQTT the daemon will use.
func WithHassOpt(hm *hass.MQTT) Opt {
	return func(d *Daemon) {
		d.hass = hm
	}
}

// WithHostAPD is required at least once and sets the hostapd.Client the daemon will use.
// Multple hostapd.Clients may be used.
func WithHostAPD(ha *hostapd.Client) Opt {
	return func(d *Daemon) {
		d.haps = append(d.haps, hap{client: ha})
	}
}

// WithLogger is optional and defines a logger for the daemon to use.
func WithLogger(l *log.Logger) Opt {
	return func(d *Daemon) {
		d.logger = l
	}
}

// WithDebounce is optionally and sets the debounce time for station connect/disconnect
// events.
func WithDebounce(db time.Duration) Opt {
	return func(d *Daemon) {
		d.db = newDebouncer(db)
	}
}

// WithHASSAutodiscovery configures whether daemon will publish MQTT autodiscovery
// messages for Home Assistant.
func WithHASSAutodiscovery(ad bool) Opt {
	return func(d *Daemon) {
		d.hassAutoDisc = ad
	}
}

// Daemon runs the main wifi-presence program loop.
type Daemon struct {
	apName       string
	hass         *hass.MQTT
	haps         []hap
	logger       *log.Logger
	db           *debouncer
	hassAutoDisc bool

	mu sync.Mutex
	// An entry here implies that the stations is configured to be tracked.
	stations map[MAC]station
}

type hap struct {
	client *hostapd.Client
	status hostapd.Status
}

type connectedStation struct {
	hapStatus hostapd.Status
	sta       hostapd.Station
}

type station struct {
	name           string
	mac            MAC
	connected      bool
	bssid          string
	connectedAt    time.Time
	disconnectedAt time.Time
}

// NewDaemon returns a Daemon, configured via the Opt arguments.
func NewDaemon(opts ...Opt) (*Daemon, error) {
	d := Daemon{
		stations: make(map[MAC]station),
	}
	for _, opt := range opts {
		opt(&d)
	}

	if d.hass == nil {
		return nil, errors.New("WithHassOpt is required")
	}

	if len(d.haps) == 0 {
		return nil, errors.New("WithHostAPD is required at least once")
	}

	// The hostapd's status is collected once at startup since the values we use,
	// such as SSID, are not expected to change. If we need to use dynamic values,
	// such as TxPower, then this will need to be re-worked.
	var err error
	for i, hap := range d.haps {
		if d.haps[i].status, err = hap.client.Status(); err != nil {
			return nil, err
		}
	}

	if d.logger == nil {
		d.logger = log.New(ioutil.Discard, "", 0)
	}
	if d.db == nil {
		d.db = newDebouncer(5 * time.Second)
	}

	return &d, nil
}

// Run starts the Daemon processing hostapd events and publishing
// to MQTT. It blocks until it encounters an error or when the context
// is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	errs := make(chan error, 1)

	// Watch for any asynchronous errors.
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errs:
			return err
		}
	})

	// Watch for configuration updates.
	eg.Go(func() error {
		d.logger.Printf("Subscribed to config topic: %q", d.hass.ConfigTopic())
		return d.hass.SubscribeConfig(ctx, func(retained bool, cfg hass.Configuration) error {
			return d.onConfigChange(ctx, retained, cfg)
		})
	})

	// Watch each hostapd for events.
	for _, hap := range d.haps {
		d.logger.Printf("Connected to AP\n  SSID: %q\n  BSSID: %q\n  CHANNEL: %02d\n  STATE: %q\n",
			hap.status.SSID,
			hap.status.BSSID,
			hap.status.Channel,
			hap.status.State,
		)

		hap := hap
		eg.Go(func() error {
			return hap.client.Attach(ctx, func(event hostapd.Event) error {
				return d.onHostapdEvent(ctx, hap, event, errs)
			})
		})
	}

	return eg.Wait()
}

const (
	staNoChange staChange = iota
	staRemoved
	staAdded
	staUpdated
)

type staChange int

func (s staChange) String() string {
	switch s {
	case staNoChange:
		return "no-change"
	case staRemoved:
		return "removed"
	case staAdded:
		return "added"
	case staUpdated:
		return "updated"
	default:
		return "?"
	}
}

func (d *Daemon) onConfigChange(ctx context.Context, retained bool, cfg hass.Configuration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Diff the new vs the current configuration.

	changes := make(map[MAC]staChange, len(cfg.Devices)+len(d.stations))
	var hasUpdates bool
	for _, devCfg := range cfg.Devices {
		var mac MAC
		if err := mac.Decode(devCfg.MAC); err != nil {
			return err
		}

		sta, ok := d.stations[mac]
		switch {
		case !ok:
			changes[mac] = staAdded
			hasUpdates = true
		case sta.name != devCfg.Name:
			changes[mac] = staUpdated
			hasUpdates = true
		default:
			changes[mac] = staNoChange
		}

		sta.name = devCfg.Name
		sta.mac = mac
		d.stations[mac] = sta
	}
	// Find previously configured stations that are no longer
	// present in the new configuration.
	for mac := range d.stations {
		if _, ok := changes[mac]; !ok {
			changes[mac] = staRemoved
			delete(d.stations, mac)
		}
	}

	var logMsg strings.Builder
	fmt.Fprintf(&logMsg, "Received config update (retained=%v):\n", retained)
	defer func() {
		d.logger.Print(logMsg.String())
	}()

	if len(changes) == 0 {
		fmt.Fprintln(&logMsg, "(no stations configured)")
		return nil
	}

	var connected map[MAC]connectedStation
	if hasUpdates {
		// Avoid calling Stations on each hostap client unless
		// necessary.
		var err error
		if connected, err = d.connectedStations(); err != nil {
			var unknown hostapd.ErrUnknownCmd
			if errors.As(err, &unknown) {
				// At this point, we can still continue. The 'connected' map will be empty, meaning
				// all stations will be considered disconnected. This is better than failing completely.
				d.logger.Print("Unable to retrieve list of connected stations. Marking all new stations (if any) as disconnected.\nSee https://github.com/awilliams/wifi-presence/#hostapd-full-version for more information")
			} else {
				return err
			}
		}
	}

	// Process each configuration change.
	for mac, change := range changes {
		sta := d.stations[mac] // May be zero value.

		switch change {
		case staNoChange:
			// Nothing to do here.

		case staUpdated:
			if d.hassAutoDisc {
				err := d.hass.RegisterDeviceTracker(ctx, hass.Discovery{
					Name: sta.name,
					MAC:  sta.mac.String(),
				})
				if err != nil {
					return err
				}
			}

		case staAdded:
			if d.hassAutoDisc {
				err := d.hass.RegisterDeviceTracker(ctx, hass.Discovery{
					Name: sta.name,
					MAC:  sta.mac.String(),
				})
				if err != nil {
					return err
				}
			}

			// Check whether this station is connected or not.
			cs, ok := connected[mac]
			if !ok {
				sta.connected = false
				d.stations[mac] = sta

				// Station is not connected.
				pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				if err := d.hass.StationNotHome(pubCtx, mac.String()); err != nil {
					return err
				}
				break
			}

			// Station is connected.

			sta.connected = true
			sta.connectedAt = time.Now().Add(-cs.sta.Connected)
			sta.bssid = cs.hapStatus.BSSID
			d.stations[mac] = sta

			d.db.cancel(mac)

			attrs := hass.Attrs{
				Name:         sta.name,
				MAC:          sta.mac.String(),
				IsConnected:  true,
				APName:       d.apName,
				SSID:         cs.hapStatus.SSID,
				BSSID:        cs.hapStatus.BSSID,
				ConnectedAt:  &sta.connectedAt,
				ConnectedFor: int(time.Since(sta.connectedAt).Seconds()),
			}

			pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := d.hass.StationHome(pubCtx, mac.String()); err != nil {
				return err
			}

			pubCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := d.hass.StationAttributes(pubCtx, mac.String(), attrs); err != nil {
				return err
			}

		case staRemoved:
			d.db.cancel(mac)

			// TODO: send disconnected state here?
			// HomeAssistant removes the devices from its registry, but other systems may depend
			// more on this state message.

			if d.hassAutoDisc {
				if err := d.hass.UnregisterDeviceTracker(ctx, mac.String()); err != nil {
					return err
				}
			}
		}

		fmt.Fprintf(&logMsg, "  %q (%s): %s\n", sta.name, mac, change.String())
	}

	return nil
}

func (d *Daemon) onHostapdEvent(ctx context.Context, hap hap, event hostapd.Event, errs chan<- error) error {
	d.logger.Printf("%s: Event %T: %q", hap.status.SSID, event, event.Raw())

	switch e := event.(type) {

	case hostapd.EventStationConnect:
		var mac MAC
		if err := mac.Decode(e.MAC); err != nil {
			return err
		}

		var shouldUpdate bool
		d.mu.Lock()
		sta, ok := d.stations[mac]
		if ok {
			shouldUpdate = !sta.connected || sta.bssid != hap.status.BSSID
			sta.bssid = hap.status.BSSID
			sta.connected = true
			sta.connectedAt = time.Now()
			d.stations[mac] = sta
		}
		d.mu.Unlock()
		if !ok {
			// Station is not being tracked.
			return nil
		}

		if d.db.cancel(mac) {
			d.logger.Printf("cancelled disconnect event for %s", mac)
		}

		if !shouldUpdate {
			break
		}

		pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := d.hass.StationHome(pubCtx, mac.String()); err != nil {
			return err
		}

		attrs := hass.Attrs{
			Name:        sta.name,
			MAC:         sta.mac.String(),
			IsConnected: true,
			APName:      d.apName,
			SSID:        hap.status.SSID,
			BSSID:       hap.status.BSSID,
			ConnectedAt: &sta.connectedAt,
			DisconnectedFor: func() int {
				if sta.disconnectedAt.IsZero() {
					return 0
				}
				return int(time.Since(sta.disconnectedAt).Seconds())
			}(),
		}

		pubCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := d.hass.StationAttributes(pubCtx, mac.String(), attrs); err != nil {
			return err
		}

	case hostapd.EventStationDisconnect:
		var mac MAC
		if err := mac.Decode(e.MAC); err != nil {
			return err
		}

		d.mu.Lock()
		sta, ok := d.stations[mac]
		if ok {
			if sta.connected && sta.bssid != hap.status.BSSID {
				// Assume that station previously connected to another AP, and
				// that this is a delayed disconnect event from the previous AP.
				d.mu.Unlock()
				d.logger.Printf("ignoring latent disconnect for %s; connected to other bssid %s", mac, sta.bssid)
				return nil
			}
			sta.connected = false
			sta.disconnectedAt = time.Now()
			d.stations[mac] = sta
		}
		d.mu.Unlock()
		if !ok {
			// Station is not being tracked.
			return nil
		}

		d.db.enqueue(mac, func() {
			pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := d.hass.StationNotHome(pubCtx, mac.String()); err != nil {
				errs <- err
				return
			}

			d.mu.Lock()
			sta, ok := d.stations[mac]
			d.mu.Unlock()

			if !ok {
				// Somehow station was removed.
				return
			}

			attrs := hass.Attrs{
				Name:           sta.name,
				MAC:            sta.mac.String(),
				IsConnected:    false,
				APName:         d.apName,
				SSID:           hap.status.SSID,
				BSSID:          hap.status.BSSID,
				ConnectedFor:   int(time.Since(sta.connectedAt).Seconds()),
				DisconnectedAt: &sta.disconnectedAt,
			}

			pubCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := d.hass.StationAttributes(pubCtx, mac.String(), attrs); err != nil {
				errs <- err
				return
			}
		})

	default:
		d.logger.Printf("%s: event not handled %T: %q", hap.status.SSID, event, event.Raw())
	}

	return nil
}

// connectedStations returns a mapping by MAC address of all connected
// stations, combining each hostap client.
func (d *Daemon) connectedStations() (map[MAC]connectedStation, error) {
	cs := make(map[MAC]connectedStation)
	for _, hap := range d.haps {
		// Stations returns a list of all the connected stations.
		stations, err := hap.client.Stations()
		if err != nil {
			return nil, err
		}
		for _, sta := range stations {
			if !sta.Associated {
				continue
			}
			var mac MAC
			if err := mac.Decode(sta.MAC); err != nil {
				return nil, err
			}
			cs[mac] = connectedStation{
				hapStatus: hap.status,
				sta:       sta,
			}
		}
	}
	return cs, nil
}
