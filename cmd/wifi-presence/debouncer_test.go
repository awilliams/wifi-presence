package main

import (
	"fmt"
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	var (
		mac      = "test-mac"
		debounce = 50 * time.Millisecond
	)

	cases := []struct {
		name     string
		do       func(c *debouncer, cb chan<- string)
		expected []string
	}{
		{
			name: "add",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "hi"
				})
			},
			expected: []string{"hi"},
		},
		{
			name: "double add",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "hi"
				})

				c.add(mac, func() {
					cb <- "repeat!"
				})
			},
			expected: []string{"hi"},
		},

		{
			name: "del",
			do: func(c *debouncer, cb chan<- string) {
				c.del(mac, func() {
					cb <- "nothing to delete"
				})
			},
			expected: nil,
		},

		{
			name: "add-del",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "+"
				})

				c.del(mac, func() {
					cb <- "-"
				})
			},
			expected: []string{"+", "-"},
		},

		{
			name: "add-del-add",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "+"
				})

				c.del(mac, func() {
					cb <- "-"
				})

				// Second add should cancel delete,
				// and since delete hasn't been executed,
				// the callback shouldn't be invoked either.
				c.add(mac, func() {
					cb <- "++"
				})
			},
			expected: []string{"+"},
		},

		{
			name: "add-del-pause-add",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "+"
				})

				c.del(mac, func() {
					cb <- "-"
				})
				// Allow delete time to be executed.
				time.Sleep(debounce + debounce/2)

				c.add(mac, func() {
					cb <- "++"
				})
			},
			expected: []string{"+", "-", "++"},
		},
		{
			name: "add-del-add-del",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "+"
				})
				c.del(mac, func() {
					cb <- "-"
				})
				// Second add should cancel delete,
				// and since delete hasn't been executed,
				// the callback shouldn't be invoked either.
				c.add(mac, func() {
					cb <- "++"
				})
				c.del(mac, func() {
					cb <- "--"
				})
			},
			expected: []string{"+", "--"},
		},

		{
			name: "del-x-64",
			do: func(c *debouncer, cb chan<- string) {
				c.add(mac, func() {
					cb <- "+"
				})

				for i := 0; i <= 64; i++ {
					msg := fmt.Sprintf("del: %d", i)
					c.del(mac, func() {
						cb <- msg
					})
				}
			},
			expected: []string{"+", "del: 0"},
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
				case <-time.After(2 * debounce):
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

func TestDebouncer_Add(t *testing.T) {
	const mac = "test-mac"

	c := newDebouncer(0)

	callback := make(chan struct{}, 1)

	c.add(mac, func() {
		callback <- struct{}{}
	})

	select {
	case <-callback:
		t.Logf("debouncer.add() callback function called")
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timeout waiting for debouncer.add() callback function to be called")
	}

	c.add(mac, func() {
		callback <- struct{}{}
	})

	select {
	case <-callback:
		t.Fatal("unexpected debouncer.add() callback function called")
	case <-time.After(250 * time.Millisecond):
		t.Logf("no unexpected debouncer.add() callback")
	}
}

func TestDebouncer_Del(t *testing.T) {
	const mac = "test-mac"
	debounce := 50 * time.Millisecond

	c := newDebouncer(debounce)

	callback := make(chan struct{}, 1)

	c.del(mac, func() {
		callback <- struct{}{}
	})

	select {
	case <-callback:
		t.Fatal("unexpected debouncer.del() callback function called")
	case <-time.After(debounce * 2):
		t.Logf("no unexpected debouncer.del() callback")
	}

	c.add(mac, nil)

	deletedAt := time.Now()
	c.del(mac, func() {
		callback <- struct{}{}
	})

	select {
	case <-callback:
		if delta := time.Since(deletedAt); delta < debounce {
			t.Fatalf("debouncer.del() callback function called after %s; expected at least %s (debounce time)", delta, debounce)
		} else {
			t.Logf("debouncer.del() callback function called after %s (debounce: %s)", delta, debounce)
		}

	case <-time.After(debounce * 2):
		t.Fatal("timeout waiting for debouncer.del() callback function to be called")
	}
}
