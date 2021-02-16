package presence

import "testing"

func TestMAC_String(t *testing.T) {
	cases := []struct {
		m        MAC
		expected string
	}{
		{MAC{}, "00:00:00:00:00:00"},
		{MAC{1, 2, 3, 4, 5, 6}, "01:02:03:04:05:06"},
		{MAC{0xDE, 0xAD, 0xBE, 0xEF, 0, 0}, "de:ad:be:ef:00:00"},
	}

	for _, tc := range cases {
		if got := tc.m.String(); got != tc.expected {
			t.Errorf("%#v.String() = %q; expected %q", tc.m, got, tc.expected)
		} else {
			t.Logf("%#v.String() = %q", tc.m, got)
		}
	}
}

func TestMAC_Decode(t *testing.T) {
	cases := []struct {
		input    string
		expected MAC
	}{
		{"00:00:00:00:00:00", MAC{}},
		{"01:02:03:04:05:06", MAC{1, 2, 3, 4, 5, 6}},
		{"DE:aD:be:EF:00:00", MAC{0xDE, 0xAD, 0xBE, 0xEF, 0, 0}},
	}

	for _, tc := range cases {
		var got MAC
		if err := got.Decode(tc.input); err != nil {
			t.Errorf("MAC.Decode(%q) err = %v; expected nil", tc.input, err)
		}

		if got != tc.expected {
			t.Errorf("MAC.Decode(%q) = %#v; expected %#v", tc.input, got, tc.expected)
		} else {
			t.Logf("MAC.Decode(%q) = %#v", tc.input, got)
		}
	}
}
