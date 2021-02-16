package main

import (
	"sync"
	"time"
)

// newDebouncer returns a debouncer that will wait debounce
// duration before executing the del callback. 0 is a valid value.
func newDebouncer(debounce time.Duration) *debouncer {
	return &debouncer{
		debounce: debounce,
		clients:  make(map[string]*time.Timer),
	}
}

// debouncer manages state for client connect and disconnect
// events. If a client is already connected, add's calllback
// will not be called. If a client is disconnected, then reconnected
// within debounce time, then del's callback will not be called.
type debouncer struct {
	// Time to wait before performing delete callbacku
	debounce time.Duration

	// A client is connected if there is a corresponding entry
	// in the following map (nil is valid). On disconnect, the client's
	// entry will be updated with a non-nil timer which is the result of
	// time.AfterFunc. This delay is used to debounce sporadic
	// disconnect->reconnect events.
	mu      sync.Mutex             // Protects following.
	clients map[string]*time.Timer // MAC -> timer
}

// add calls the onAdd callback if the given mac is not present
// in the cache. It also cancels any debounced deletes.
func (d *debouncer) add(mac string, onAdd func()) {
	d.mu.Lock()

	if t, ok := d.clients[mac]; ok {
		// Client has already been added (regardless of if
		// t is nil). Callback will not be called.

		if t != nil {
			// Client was slated for deletion, but was
			// re-added. Stop deletion callback.
			t.Stop()
			d.clients[mac] = nil
		}
		d.mu.Unlock()
		return
	}

	// Client is added.

	d.clients[mac] = nil
	d.mu.Unlock()

	if onAdd != nil {
		onAdd()
	}
}

// del calls the onDel callback after debounce duration. The callback
// may be cancelled if add is called before the debounce duration. If del
// was already called within the debounce time, then this call to del
// does nothing (allowing previous del to complete).
func (d *debouncer) del(mac string, onDel func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	t, ok := d.clients[mac]
	if !ok {
		// Client has already been deleted. Nothing
		// more to do.
		return
	}

	if t != nil {
		// A previous deletion is set. Allow it to
		// complete. Nothing left to do.
		return
	}

	// Run delete callback after configured debounce time.
	d.clients[mac] = time.AfterFunc(d.debounce, func() {
		d.mu.Lock()

		if _, ok := d.clients[mac]; !ok {
			// Client has been deleted since timer was set.
			// Nothing left to do.
			d.mu.Unlock()
			return
		}

		delete(d.clients, mac)
		d.mu.Unlock()

		if onDel != nil {
			onDel()
		}
	})
}
