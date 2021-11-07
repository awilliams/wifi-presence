// This program starts a mock hostapd service listening on a Unix socket.
// It can be controlled via prompts on STDIN.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/awilliams/wifi-presence/internal/hostapd/hostapdtest"
)

func main() {
	sockPath := flag.String("sockPath", "", "Path to mock hostapd Unix socket")
	macAddr := flag.String("mac", "BE:EF:00:00:FA:CE", "Device MAC address")
	ssid := flag.String("ssid", "Test AP", "Mock SSID")
	bssid := flag.String("bssid", "54:65:73:74:41:50", "Mock BSSID")
	connected := flag.Bool("connected", false, "If true, then station is initially connected")

	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if *sockPath == "" {
		*sockPath = "hostapd.sock"
	}
	hap, err := hostapdtest.NewHostAPD(*sockPath)
	if err != nil {
		bail(err)
	}
	defer func() {
		hap.Close()
		os.Remove(hap.Addr)
	}()

	var stations []hostapdtest.StationResp
	if *connected {
		stations = append(stations, hostapdtest.StationResp{
			Assoc:  true,
			Signal: 11,
			MAC:    *macAddr,
		})
	}
	handler := hostapdtest.DefaultHostAPDHandler(hostapdtest.StatusResp{
		SSID:       *ssid,
		BSSID:      *bssid,
		State:      "ENABLED",
		Channel:    42,
		MaxTxPower: 11,
	}, stations)

	detached := make(chan struct{})
	handler.OnDetach(func() { close(detached) })

	events := make(chan string)
	attached := make(chan struct{})
	handler.OnAttach(func() <-chan string {
		log.Println("wifi-presence attached")
		close(attached)
		return events
	})

	go func() {
		if err := hap.Serve(handler); err != nil {
			bail(err)
		}
	}()

	const instructions = `
Enter the following number for the corresponding action:
1:      Send CONNECT event for %q
2:      Send DISCONNECT event for %q
3:      Send TERMINATING event
q:      Exit

Command: `

	fmt.Printf("Created test HostAP socket at: %s\nWaiting for wifi-presence to connect...\n", hap.Addr)
	select {
	case <-attached:
		fmt.Printf(instructions, *macAddr, *macAddr)
	case <-ctx.Done():
		return
	}

	sendEvent := func(event string) error {
		select {
		case events <- event:
			log.Printf("> Sent: %q\n", event)
			return nil
		case <-time.After(time.Second):
			return errors.New("timeout sending event")
		}
	}

	lines := readLines(ctx)
	for {
		select {
		case line := <-lines:
			switch line {
			case "1":
				if err := sendEvent(fmt.Sprintf("AP-STA-CONNECTED %s", *macAddr)); err != nil {
					bail(err)
				}

			case "2":
				if err := sendEvent(fmt.Sprintf("AP-STA-DISCONNECTED %s", *macAddr)); err != nil {
					bail(err)
				}

			case "3":
				if err := sendEvent("CTRL-EVENT-TERMINATING"); err != nil {
					bail(err)
				}
				time.Sleep(time.Second)
				return

			case "q", "Q", "exit":
				return

			default:
				log.Printf("Unrecognized number %q\n", line)
			}

			fmt.Printf(instructions, *macAddr, *macAddr)

		case <-ctx.Done():
			return

		case <-detached:
			log.Println("wifi-presence detached. Exiting...")
			return
		}
	}
}

func readLines(ctx context.Context) <-chan string {
	lines := make(chan string)
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		<-ctx.Done()
		os.Stdin.Close()
	}()
	go func() {
		defer close(lines)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	return lines
}

func bail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}
