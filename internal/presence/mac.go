package presence

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// MAC is a hardware MAC address.
type MAC [6]byte

// String returns the address in a "XX:XX:XX:XX:XX" formatted string
// (lower-case letters).
func (m MAC) String() string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", m[0], m[1], m[2], m[3], m[4], m[5])
}

// Decode converts a string of form "XX:XX:XX:XX:XX" to a MAC.
func (m *MAC) Decode(s string) error {
	s = strings.Replace(s, ":", "", 5)
	b, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	if len(b) != len(m) {
		return fmt.Errorf("invalid MAC length %d; expected %d from %q", len(b), len(m), s)
	}

	copy(m[:], b)

	return nil
}

func (m MAC) MarshalJSON() ([]byte, error) {
	return []byte(`"` + m.String() + `"`), nil
}

func (m *MAC) UnmarshalJSON(b []byte) error {
	var decoded MAC
	if len(b) < 2 {
		return errors.New("invalid MAC: too short")
	}
	if err := decoded.Decode(string(b[1 : len(b)-1])); err != nil {
		return err
	}
	*m = decoded
	return nil
}
