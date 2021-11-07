package presence

import (
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	var (
		mac      = MAC{0xFF, 0xBE, 0xEF, 0x00, 0x00, 0x00}
		debounce = 50 * time.Millisecond
	)

	cases := []struct {
		name     string
		do       func(c *debouncer, cb chan<- string)
		expected []string
	}{
		{
			name: "enq",
			do: func(c *debouncer, cb chan<- string) {
				c.enqueue(mac, func() {
					cb <- "hi"
				})
			},
			expected: []string{"hi"},
		},
		{
			name: "enq-enq",
			do: func(c *debouncer, cb chan<- string) {
				c.enqueue(mac, func() {
					cb <- "hi"
				})

				c.enqueue(mac, func() {
					cb <- "repeat!"
				})
			},
			expected: []string{"hi"},
		},

		{
			name: "cancel",
			do: func(c *debouncer, _ chan<- string) {
				t.Logf("cancel: %v", c.cancel(mac))
			},
			expected: nil,
		},

		{
			name: "enq-cancel",
			do: func(c *debouncer, cb chan<- string) {
				c.enqueue(mac, func() {
					cb <- "+"
				})

				t.Logf("cancel: %v", c.cancel(mac))
			},
			expected: nil,
		},

		{
			name: "enq-cancel-enq",
			do: func(c *debouncer, cb chan<- string) {
				c.enqueue(mac, func() {
					cb <- "1"
				})

				t.Logf("cancel: %v", c.cancel(mac))

				c.enqueue(mac, func() {
					cb <- "2"
				})
			},
			expected: []string{"2"},
		},

		{
			name: "enque-pause-cancel-enq",
			do: func(c *debouncer, cb chan<- string) {
				c.enqueue(mac, func() {
					cb <- "+"
				})

				// Allow delete time to be executed.
				time.Sleep(debounce + debounce/2)
				t.Logf("cancel: %v", c.cancel(mac))

				c.enqueue(mac, func() {
					cb <- "++"
				})
			},
			expected: []string{"+", "++"},
		},
		{
			name: "enq-enq-cancel",
			do: func(c *debouncer, cb chan<- string) {
				c.enqueue(mac, func() {
					cb <- "+"
				})
				c.enqueue(mac, func() {
					cb <- "++"
				})
				t.Logf("cancel: %v", c.cancel(mac))
			},
			expected: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newDebouncer(debounce)
			callback := make(chan string, 1)
			go tc.do(c, callback)

			for _, expected := range tc.expected {
				select {
				case got := <-callback:
					if got != expected {
						t.Fatalf("callback performed, got %q; expected %q", got, expected)
					} else {
						t.Logf("callback performed, got %q", got)
					}
				case <-time.After(4 * debounce):
					t.Fatal("timeout waiting for callback")
				}
			}

			select {
			case got := <-callback:
				t.Fatalf("unexpected callback performed, got %q", got)
			case <-time.After(2 * debounce):
				t.Log("no unexpected callback")
			}
		})
	}
}

func TestDebouncer_Enqueue(t *testing.T) {
	mac := MAC{0xFF, 0xBE, 0xEF, 0x00, 0x00, 0x00}

	c := newDebouncer(0)

	callback := make(chan struct{}, 1)

	enqueuedAt := time.Now()
	result := c.enqueue(mac, func() {
		callback <- struct{}{}
	})
	if !result {
		t.Fatalf("c.enqueue = %v; expected true", result)
	}

	select {
	case <-callback:
		delta := time.Since(enqueuedAt)
		t.Logf("debouncer.add() callback function called after %s", delta)
		if delta > 100*time.Millisecond {
			t.Error("unexpectedly large delta since callback was enqueued")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timeout waiting for debouncer.add() callback function to be called")
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	mac := MAC{0xFF, 0xBE, 0xEF, 0x00, 0x00, 0x00}
	debounce := 50 * time.Millisecond

	c := newDebouncer(debounce)

	callback := make(chan struct{}, 1)

	c.enqueue(mac, func() {
		callback <- struct{}{}
	})
	if result := c.cancel(mac); !result {
		t.Fatalf("c.cancel() = %v; expected true", result)
	}

	select {
	case <-callback:
		t.Fatal("unexpected debouncer.del() callback function called")
	case <-time.After(debounce * 2):
		t.Logf("no unexpected debouncer.del() callback")
	}
}
