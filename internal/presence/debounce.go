package presence

import (
	"sync"
	"time"
)

// newDebouncer returns a debouncer that will wait the given
// duration before executing any enqueued callback. 0 is a valid value.
func newDebouncer(debounce time.Duration) *debouncer {
	return &debouncer{
		debounce: debounce,
		queue:    make(map[MAC]*time.Timer),
	}
}

type debouncer struct {
	// Time to wait before performing delete callback.
	debounce time.Duration

	mu    sync.Mutex // Protects following.
	queue map[MAC]*time.Timer
}

// cancel any enqueued callback for the given MAC.
func (c *debouncer) cancel(mac MAC) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cancelled bool
	if t, ok := c.queue[mac]; ok {
		// mac had a callback enqueue, but was
		// cancelled. Stop callback timer.
		cancelled = t.Stop()
	}
	delete(c.queue, mac)
	return cancelled
}

// enqueue executes the callback after debounce duration. The callback
// may be cancelled if cancel is called before the debounce duration. If
// a callback is already queued for this MAC, then this call does nothing
// (allowing previously enqueued callback complete).
func (c *debouncer) enqueue(mac MAC, cb func()) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.queue[mac]; ok {
		// There's a callback already enqueued for this MAC.
		// Nothing more to do.
		return false
	}

	// Run callback after configured debounce time.
	c.queue[mac] = time.AfterFunc(c.debounce, func() {
		c.mu.Lock()

		if _, ok := c.queue[mac]; !ok {
			// This callback has been cancelled since timer was set.
			// Nothing more to do.
			c.mu.Unlock()
			return
		}
		delete(c.queue, mac)
		c.mu.Unlock()

		if cb != nil {
			cb()
		}
	})
	return true
}
