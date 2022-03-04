package hostapd

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Status holds information about the WPA. This is a subset
// of all the fields returned from the control interface.
// More info:
// https://w1.fi/wpa_supplicant/devel/ctrl_iface_page.html#ctrl_iface_STATUS
type Status struct {
	State      string
	Channel    int
	MaxTxPower int
	SSID       string
	BSSID      string
}

// parse parses the hostapd control interface
// message representing status and updates s.
func (s *Status) parse(p []byte) error {
	var (
		err      error
		line     string
		parts    []string
		key, val string
	)

	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line = scanner.Text()

		parts = strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid status response line %q", line)
		}
		key, val = parts[0], parts[1]

		switch key {
		case "state":
			s.State = val

		case "channel":
			if s.Channel, err = strconv.Atoi(val); err != nil {
				return err
			}

		case "max_txpower":
			if s.MaxTxPower, err = strconv.Atoi(val); err != nil {
				return err
			}

		case "ssid[0]":
			if strings.HasPrefix(val, "\\x") {
				val = strings.ReplaceAll(val, "\\x", "")
				h, err := hex.DecodeString(val)
				if err != nil {
					return err
				}
				val = string(h)
			}
			s.SSID = val

		case "bssid[0]":
			s.BSSID = val
		}
	}

	return scanner.Err()
}
