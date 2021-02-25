package presence

import (
	"context"
	"testing"
	"time"
)

func TestPresence(t *testing.T) {
	var (
		mac1 = MAC{0xFA, 0xCE, 0, 0, 0, 0}
		mac2 = MAC{0xBE, 0xEF, 0, 0, 0, 0}
	)
	now := time.Now()

	cases := []struct {
		name         string
		events       []StationEvent
		expectations map[int]bool // event index -> onChange
	}{
		{
			name: "other-mac",
		},
		{
			name: "other-mac",
			events: []StationEvent{
				{
					MAC:    MAC{},
					Action: ActionConnect,
				},
			},
		},

		{
			name: "connect",
			events: []StationEvent{
				{
					MAC:    mac1,
					Action: ActionConnect,
				},
			},
			expectations: map[int]bool{
				0: true,
			},
		},
		{
			name: "disconnect",
			events: []StationEvent{
				{
					MAC:    mac1,
					Action: ActionDisconnect,
				},
			},
			// No event since "presence=false" is starting state.
		},
		{
			name: "connect-disconnect",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionDisconnect,
					Timestamp: now.Add(time.Second),
				},
			},
			expectations: map[int]bool{
				0: true,
				1: false,
			},
		},
		{
			name: "multi-connect-partial-disconnect",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac2,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionDisconnect,
					Timestamp: now.Add(time.Minute),
				},
			},
			expectations: map[int]bool{
				0: true,
				// No state changes after first event.
			},
		},
		{
			name: "multi-connect-full-disconnect",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac2,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionDisconnect,
					Timestamp: now.Add(time.Second),
				},
				{
					MAC:       mac2,
					AP:        "A",
					Action:    ActionDisconnect,
					Timestamp: now.Add(time.Second),
				},
			},
			expectations: map[int]bool{
				0: true,
				3: false,
			},
		},

		{
			name: "roaming-disconnect",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "B",
					Action:    ActionDisconnect,
					Timestamp: now.Add(time.Second),
				},
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionDisconnect,
					Timestamp: now.Add(2 * time.Second),
				},
			},
			expectations: map[int]bool{
				0: true,
				// No event at index 1 because diff AP.
				2: false,
			},
		},
		{
			name: "roaming-connects",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "B",
					Action:    ActionConnect,
					Timestamp: now.Add(time.Minute),
				},
			},
			expectations: map[int]bool{
				0: true,
				// At 1 index, no state change.
			},
		},

		{
			name: "roaming-connect-out-of-order",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "B",
					Action:    ActionConnect,
					Timestamp: now.Add(-1 * time.Minute),
				},
			},
			expectations: map[int]bool{
				0: true,
				// At 1 index, no state change.
			},
		},

		{
			name: "same-AP-diff-SSID",
			events: []StationEvent{
				{
					MAC:       mac1,
					AP:        "A",
					SSID:      "ONE",
					Action:    ActionConnect,
					Timestamp: now,
				},
				{
					MAC:       mac1,
					AP:        "A",
					SSID:      "TWO",
					Action:    ActionDisconnect,
					Timestamp: now.Add(1 * time.Minute),
				},
			},
			expectations: map[int]bool{
				0: true,
				// At 1 index, no state change.
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			p := NewPresence()
			w := p.Watch(ctx, []MAC{mac1, mac2})

			for i, e := range tc.events {
				var expected, ok bool
				if tc.expectations != nil {
					expected, ok = tc.expectations[i]
				}

				p.HandleStationEvent(e)

				select {
				case got := <-w:
					if !ok {
						t.Fatalf("unexpected onChange(%v) called at i:%d", got, i)
					}
					if got != expected {
						t.Fatalf("onChange(%v) called; expected onChange(%v) at i:%d", got, expected, i)
					}
					t.Logf("onChange(%v) called at i:%d", got, i)

					//case <-time.After(time.Second):
				default:
					if ok {
						t.Fatalf("onChange() not called; expected onChange(%v) at i:%d", expected, i)
					}
					t.Logf("onChange() not called at i:%d", i)
				}
			}
		})
	}
}

func TestPresence_WatchCancel(t *testing.T) {
	type watcher struct {
		w         <-chan bool
		cancel    func()
		cancelled bool
	}

	mac1 := MAC{0xFA, 0xCE, 0, 0, 0, 0}
	p := NewPresence()

	// Create arbitrary number of watchers, all
	// watching same MAC address.
	watchers := make([]*watcher, 8)
	for i := range watchers {
		ctx, cancel := context.WithCancel(context.Background())
		watchers[i] = &watcher{
			w:      p.Watch(ctx, []MAC{mac1}),
			cancel: cancel,
		}
	}

	action := ActionDisconnect
	// Cancel each watcher.
	for i, toCancelWatcher := range watchers {
		toCancelWatcher.cancel()
		toCancelWatcher.cancelled = true
		// Wait for goroutine to react to cancellation.
		select {
		case _, ok := <-toCancelWatcher.w:
			if ok {
				t.Fatal("watch channel open; expected to be closed")
			}
		case <-time.After(time.Second):
			t.Fatal("watch channel blocked; expected to be closed")
		}

		t.Logf("Iteration: %d", i)
		// Alternative connect & disconnect actions.
		p.HandleStationEvent(StationEvent{
			MAC: mac1,
			Action: func() string {
				switch action {
				case ActionConnect:
					action = ActionDisconnect
				case ActionDisconnect:
					action = ActionConnect
				}
				return action
			}(),
		})

		// Verify each watcher either receives expected value
		// or has a closed channel, depending on whether it's
		// been cancelled or not.
		for _, w := range watchers {
			var (
				expectReceive = !w.cancelled
				expectValue   = action == ActionConnect
			)
			select {
			case got, ok := <-w.w:
				if expectReceive {
					if !ok {
						t.Fatal("watch channel closed; expected to be open")
					}
					if got != expectValue {
						t.Fatalf("watch channel received %v; expected %v", got, expectValue)
					}
					t.Logf("watch channel received %v", got)
				} else {
					if ok {
						t.Fatal("watch channel not closed; expected to be closed")
					}
					t.Logf("watch channel closed")
				}
			default:
				t.Fatal("watch channel blocked; expected a read")
			}
		}
	}
}

func TestPresence_WatchMultiMAC(t *testing.T) {
	var (
		p    = NewPresence()
		mac1 = MAC{0xFA, 0xCE, 0, 0, 0, 0}
		mac2 = MAC{0xFA, 0xCE, 0, 0, 0, 1}
		mac3 = MAC{0xFA, 0xCE, 0, 0, 0, 2}
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w1 := p.Watch(ctx, []MAC{mac1})
	w2 := p.Watch(ctx, []MAC{mac1, mac2})
	w3 := p.Watch(ctx, []MAC{mac1, mac2, mac3})

	p.HandleStationEvent(StationEvent{
		MAC:    mac1,
		Action: ActionConnect,
	})

	expected := true
	for _, w := range []<-chan bool{w1, w2, w3} {
		select {
		case got, ok := <-w:
			if !ok {
				t.Fatal("channel closed; expected to be open")
			}
			if got != expected {
				t.Fatalf("channel received %v; expected %v", got, expected)
			}
		default:
			t.Fatal("channel blocked; expected to have read")
		}
	}

	p.HandleStationEvent(StationEvent{
		MAC:    mac1,
		Action: ActionDisconnect,
	})

	expected = false
	for _, w := range []<-chan bool{w1, w2, w3} {
		select {
		case got, ok := <-w:
			if !ok {
				t.Fatal("channel closed; expected to be open")
			}
			if got != expected {
				t.Fatalf("channel received %v; expected %v", got, expected)
			}
		default:
			t.Fatal("channel blocked; expected to have read")
		}
	}

	p.HandleStationEvent(StationEvent{
		MAC:    mac2,
		Action: ActionConnect,
	})

	for _, w := range []<-chan bool{w1} {
		select {
		case <-w:
			t.Fatal("channel unblocked; expected to be blocked")
		default:
		}
	}

	expected = true
	for _, w := range []<-chan bool{w2, w3} {
		select {
		case got, ok := <-w:
			if !ok {
				t.Fatal("channel closed; expected to be open")
			}
			if got != expected {
				t.Fatalf("channel received %v; expected %v", got, expected)
			}
		default:
			t.Fatal("channel blocked; expected to have read")
		}
	}

}
