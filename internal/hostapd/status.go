package hostapd

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
			if s.SSID, err = decodeSSID(scanner.Bytes()); err != nil {
				return err
			}
		case "bssid[0]":
			s.BSSID = val
		}
	}

	return scanner.Err()
}

// decodeSSID converts the hostap encoding of the SSID into a string,
// respecting the special escape sequences for hex and other characters.
// See printf_encode for more encoding info:
// https://w1.fi/cgit/hostap/tree/src/utils/common.c?id=b20991da6936a1baae9f2239ee127610a6f5335d#n477
func decodeSSID(v []byte) (string, error) {
	r := bytes.NewReader(v)
	var s strings.Builder
	s.Grow(len(v))

	for {
		c, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		// Check for escape character.
		if c != '\\' {
			s.WriteByte(c)
			continue
		}

		// Read control character.
		c, err = r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return "", fmt.Errorf("dangling escape: %w", err)
		}

		switch c {
		case '"':
			s.WriteRune('"')
		case '\\':
			s.WriteRune('\\')
		case 'e':
			s.WriteRune('\033')
		case 'n':
			s.WriteRune('\n')
		case 'r':
			s.WriteRune('\r')
		case 't':
			s.WriteRune('\t')
		case 'x': // Hex
			if _, err = io.Copy(&s, hex.NewDecoder(io.LimitReader(r, 2))); err != nil {
				return "", err
			}
		default:
			// Invalid or unknown escape char.
			s.WriteByte(c)
		}
	}

	return s.String(), nil
}
