package hostapd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Station contains information about a WiFi station (client).
type Station struct {
	MAC        string
	Associated bool
	RxBytes    int64
	TxBytes    int64
	Connected  time.Duration
	Inactive   time.Duration
	Signal     int
}

// parse parses the hostapd control interface
// message representing a station and updates s.
func (s *Station) parse(p []byte) error {
	if msg := string(p); msg == "FAIL\n" {
		return errors.New("station: fail")
	}

	var (
		err      error
		line     string
		parts    []string
		key, val string
		i        int
	)

	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line = scanner.Text()

		// First line of the response is the MAC address.
		if i++; i == 1 {
			if !isMAC(line) {
				return fmt.Errorf("invalid station MAC address: %q", line)
			}
			s.MAC = line
			continue
		}

		// All following lines are in "key=val" format.

		parts = strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid station response line %q", line)
		}
		key, val = parts[0], parts[1]

		switch key {
		case "flags":
			s.Associated = strings.Contains(val, "[ASSOC]")

		case "rx_bytes":
			if s.RxBytes, err = strconv.ParseInt(val, 10, 64); err != nil {
				return err
			}

		case "tx_bytes":
			if s.TxBytes, err = strconv.ParseInt(val, 10, 64); err != nil {
				return err
			}

		case "connected_time":
			seconds, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			s.Connected = time.Second * time.Duration(seconds)

		case "inactive_msec":
			msec, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			s.Inactive = time.Millisecond * time.Duration(msec)

		case "signal":
			if s.Signal, err = strconv.Atoi(val); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}
