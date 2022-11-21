package hostapd

import "testing"

func TestParseEvent(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected Event
	}{
		{
			name:  "connect",
			input: "<3>AP-STA-CONNECTED 04:ab:00:12:34:56",
			expected: EventStationConnect{
				raw: "<3>AP-STA-CONNECTED 04:ab:00:12:34:56",
				MAC: "04:ab:00:12:34:56",
			},
		},
		{
			name:  "connect with algo",
			input: "AP-STA-CONNECTED 04:ab:00:12:34:56 auth_alg=open",
			expected: EventStationConnect{
				raw: "AP-STA-CONNECTED 04:ab:00:12:34:56 auth_alg=open",
				MAC: "04:ab:00:12:34:56",
			},
		},
		{
			name:  "connect-lvl1",
			input: "<1>AP-STA-CONNECTED 04:ab:00:12:34:56",
			expected: EventStationConnect{
				raw: "<1>AP-STA-CONNECTED 04:ab:00:12:34:56",
				MAC: "04:ab:00:12:34:56",
			},
		},
		{
			name:  "connect-no-lvl",
			input: "AP-STA-CONNECTED 04:ab:00:12:34:56",
			expected: EventStationConnect{
				raw: "AP-STA-CONNECTED 04:ab:00:12:34:56",
				MAC: "04:ab:00:12:34:56",
			},
		},
		{
			name:  "disconnect",
			input: "<3>AP-STA-DISCONNECTED 04:ab:00:12:34:56",
			expected: EventStationDisconnect{
				raw: "<3>AP-STA-DISCONNECTED 04:ab:00:12:34:56",
				MAC: "04:ab:00:12:34:56",
			},
		},
		{
			name:     "terminating",
			input:    "<3>CTRL-EVENT-TERMINATING",
			expected: EventTerminating("<3>CTRL-EVENT-TERMINATING"),
		},
		{
			name:     "unrecognized",
			input:    "<3>TEST",
			expected: EventUnrecognized("<3>TEST"),
		},
		{
			name:     "strange",
			input:    "?",
			expected: EventUnrecognized("?"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEvent(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.expected {
				t.Fatalf("got:%#v\nexpected:%#v", got, tc.expected)
			}
			t.Logf("got:%#v", got)
		})
	}
}
