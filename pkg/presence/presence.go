package presence

import (
	"context"
	"sync"
)

// NewPresence returns a Presence instance.
func NewPresence() *Presence {
	return &Presence{
		clients: make(map[MAC]*client),
	}
}

// Presence tracks WiFi client state, including whtn
// a client roams between APs.
// The HandleStationEvent method should be used to process
// events received from wifi-presence, e.g. from MQTT messages.
// The Watch method can be used to subscribe to presence state changes
// for one or more MAC addresses.
type Presence struct {
	mu       sync.RWMutex // Protects following
	clients  map[MAC]*client
	presence bool
}

type client struct {
	mac    MAC
	latest StationEvent
	notify map[chan<- *sync.WaitGroup]struct{}
}

// Watch the collection of MAC addresses for presence state changes.
// If all of the MAC addresses become absent, then false will be sent
// to the channel. If any one of the MAC addresses become present, then
// true will be sent to the channel.
// The initial state is not sent.
// The channel is closed if/when the context is cancelled.
func (p *Presence) Watch(ctx context.Context, macs []MAC) <-chan bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	ret := make(chan bool, 1)

	n := make(chan *sync.WaitGroup, 1)
	// Register the notify channel for each MAC address.
	for _, m := range macs {
		c, ok := p.clients[m]
		if !ok {
			c = &client{
				mac:    m,
				notify: make(map[chan<- *sync.WaitGroup]struct{}),
			}
			p.clients[m] = c
		}
		c.notify[n] = struct{}{}
	}

	go func() {
		defer func() {
			// Cleanup and unregister the notify channel.
			p.mu.Lock()
			defer p.mu.Unlock()
			for _, m := range macs {
				c, ok := p.clients[m]
				if !ok {
					panic("presence: missing map entry")
				}
				delete(c.notify, n)
				if len(c.notify) == 0 {
					delete(p.clients, m)
				}
			}
			// Drain n in case of a race between n and the context channel.
			select {
			case wg := <-n:
				wg.Done()
			default:
			}
			close(n)
			close(ret)
		}()

		var (
			presence     bool
			nextPresence bool
		)

		for {
			select {
			case <-ctx.Done():
				return
			case wg := <-n:
				// A MAC address changed state.
				// Determine if the collection (macs) of MAC
				// addresses changed state.
				nextPresence = false
				p.mu.RLock()
				for _, m := range macs {
					c, ok := p.clients[m]
					if !ok {
						continue
					}
					if c.latest.IsConnect() {
						nextPresence = true
						break
					}
				}
				p.mu.RUnlock()
				if presence != nextPresence {
					presence = nextPresence
					ret <- presence
				}
				wg.Done()
			}
		}
	}()

	return ret
}

// HandleStationEvent updates the presence state using the given station event.
// Typically, these events are received from MQTT as published by the
// wifi-presence program.
func (p *Presence) HandleStationEvent(event StationEvent) {
	p.mu.Lock()

	client, ok := p.clients[event.MAC]
	if !ok {
		// This MAC address isn't being tracked.
		p.mu.Unlock()
		return
	}

	prev := client.latest.IsConnect()

	switch event.Action {
	case ActionConnect:
		if event.Timestamp.Before(client.latest.Timestamp) {
			// Out-of-order connect event.
			break
		}

		client.latest = event

	case ActionDisconnect:
		if !(event.AP == client.latest.AP && event.SSID == client.latest.SSID) {
			// Disconnect from previous AP.
			break
		}

		client.latest = event
	}

	if prev == client.latest.IsConnect() {
		// No change to client's connection state.
		p.mu.Unlock()
		return
	}

	var wg sync.WaitGroup
	for n := range client.notify {
		wg.Add(1)
		n <- &wg
	}
	p.mu.Unlock()
	wg.Wait()
}
