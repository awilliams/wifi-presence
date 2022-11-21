package hostapd

import (
	"regexp"
)

var macRegexp = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`)

const macLength = 2*6 + 5

// validateMAC returns an false if v is not a valid MAC address
// in XX:XX:XX:XX:XX:XX format.
func isMAC(v string) bool {
	return macRegexp.MatchString(v)
}
