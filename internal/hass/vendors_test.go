package hass

import "testing"

func TestVendorByMAC(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"FF:FF:FF:FF:FF:FF", ""},
		{"FF:FF", ""},
		{"00:03:93", vendorApple},
		{"00:03:93:00:00:FF", vendorApple},
		{"CC:20:E8:FF:FF:FF", vendorApple},
		{"00:1A:11", vendorGoogle},
		{"00:00:F0", vendorSamsung},
	}

	for _, tc := range cases {
		if got := VendorByMAC(tc.input); got != tc.expected {
			t.Errorf("VendorByMac(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
