package main

import (
	"context"
	"log"
	"time"

	"github.com/awilliams/wifi-presence/pkg/hostapd"
	"github.com/awilliams/wifi-presence/pkg/presence"
	"golang.org/x/sync/errgroup"
)

// newApp creates an app that uses the hostapd client to receive
// station events and the MQTT client to publish the events. Debounce duration
// controls how long to wait until considering a client disconnected. 0 is a valid value.
func newApp(apName string, debounce time.Duration, m *mqttClient, h *hostapd.Client) (*app, error) {
	apStatus, err := h.Status()
	if err != nil {
		return nil, err
	}
	log.Printf("AP status:\n SSID: %q\n BSSID: %q\n CHANNEL: %02d\n STATE: %q\n",
		apStatus.SSID,
		apStatus.BSSID,
		apStatus.Channel,
		apStatus.State,
	)

	return &app{
		apName:   apName,
		apStatus: apStatus,
		d:        newDebouncer(debounce),
		mqtt:     m,
		hostapd:  h,
	}, nil
}

// app monitors a single hostapd and publishes its events to MQTT.
type app struct {
	apName   string         // AP name (arbitrary)
	apStatus hostapd.Status // Initial status of AP

	d *debouncer

	mqtt    *mqttClient
	hostapd *hostapd.Client
}

// publishAll requests all connected clients from the hostapd interface
// and publishes each via the MQTT client.
func (a *app) publishAll(ctx context.Context) error {
	stations, err := a.hostapd.Stations()
	if err != nil {
		return err
	}

	for _, sta := range stations {
		log.Printf("%s: Connected Station:\n MAC: %q\n ASSOCIATED: %v\n CONNECTED: %s\n Tx/Rx BYTES: %d/%d\n SIGNAL: %d",
			a.apStatus.SSID,
			sta.MAC,
			sta.Associated,
			sta.Connected,
			sta.TxBytes, sta.RxBytes,
			sta.Signal,
		)

		if !sta.Associated {
			continue
		}
		stationEvent := presence.StationEvent{
			AP:        a.apName,
			SSID:      a.apStatus.SSID,
			BSSID:     a.apStatus.BSSID,
			Timestamp: time.Now(),
			Action:    presence.ActionConnect,
		}
		if err = stationEvent.MAC.Decode(sta.MAC); err != nil {
			return err
		}

		pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = a.mqtt.stationEvent(pubCtx, stationEvent)
		cancel()
		if err != nil {
			return err
		}

		// No callback function needed, since we know all
		// stations are new at this point.
		// TODO: return stations instead of adding them here.
		a.d.add(sta.MAC, nil)
	}

	return nil
}

// run attaches to the hostapd's events and publishes connect and
// disconnect events via the MQTT client. The method runs until
// error or the context is cancelled.
func (a *app) run(ctx context.Context) error {
	if err := a.publishAll(ctx); err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	pubErrs := make(chan error, 1)

	// Monitor pubErrs channel for any async errors.
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case err := <-pubErrs:
			return err
		}
	})

	eg.Go(func() error {
		return a.hostapd.Attach(ctx, func(event hostapd.Event) error {
			log.Printf("%s: Unsolicited message (%T): %q", a.apStatus.SSID, event, event.Raw())

			stationEvent := presence.StationEvent{
				AP:        a.apName,
				SSID:      a.apStatus.SSID,
				BSSID:     a.apStatus.BSSID,
				Timestamp: time.Now(),
			}

			switch e := event.(type) {
			case hostapd.EventStationConnect:
				stationEvent.Action = presence.ActionConnect
				if err := stationEvent.MAC.Decode(e.MAC); err != nil {
					return err
				}

				a.d.add(e.MAC, func() {
					pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()
					if err := a.mqtt.stationEvent(pubCtx, stationEvent); err != nil {
						pubErrs <- err
					}
				})

			case hostapd.EventStationDisconnect:
				stationEvent.Action = presence.ActionDisconnect
				if err := stationEvent.MAC.Decode(e.MAC); err != nil {
					return err
				}

				a.d.del(e.MAC, func() {
					pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()
					if err := a.mqtt.stationEvent(pubCtx, stationEvent); err != nil {
						pubErrs <- err
					}
				})
			}

			return nil
		})
	})

	return eg.Wait()
}
