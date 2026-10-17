package hass

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAttrsInZones(t *testing.T) {
	testCases := []struct {
		name  string
		attrs Attrs
		want  string
	}{
		{
			name:  "connected",
			attrs: Attrs{InZones: []string{ZoneHome}},
			want:  `"in_zones":["zone.home"]`,
		},
		{
			name:  "disconnected",
			attrs: Attrs{InZones: []string{}},
			want:  `"in_zones":[]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.attrs)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), tc.want) {
				t.Fatalf("got %s; want it to contain %s", b, tc.want)
			}
		})
	}
}

// TestAttrsInZonesNotOmitted ensures in_zones is never omitted, since
// Home Assistant treats a missing in_zones differently from an empty one.
func TestAttrsInZonesNotOmitted(t *testing.T) {
	b, err := json.Marshal(Attrs{})
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(b, &attrs); err != nil {
		t.Fatal(err)
	}
	if _, ok := attrs["in_zones"]; !ok {
		t.Fatalf("got %s; want in_zones key present", b)
	}
}
