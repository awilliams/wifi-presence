package hostapd

import "testing"

func TestIsMAC(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"ab:CD:12:00:00:00", true},
		{"", false},
		{"ab:CD:12:00:00:00 ", false},
		{"ab:CD:12:00:00", false},
		{"ab:CD:12:00:00:00:", false},
		{"ab:CD:12:00:00:0X", false},
		{"XX:XX:XX:XX:XX:XX", false},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			got := isMAC(tc.input)
			t.Logf("isMAC(%q) = %v", tc.input, got)
			if got != tc.expected {
				t.Fatalf("expected %v", tc.expected)
			}
		})
	}
}
