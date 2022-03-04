package hostapd

import (
	"testing"
	"time"
)

func TestStationParse(t *testing.T) {
	var got Station
	if err := got.parse([]byte(stationMsg)); err != nil {
		t.Fatal(err)
	}

	expected := Station{
		MAC:        "fa:ce:aa:bb:12:34",
		Associated: true,
		RxBytes:    13716568000,
		TxBytes:    90628298,
		Inactive:   590 * time.Millisecond,
		Signal:     -64,
	}
	if got != expected {
		t.Fatalf("got:\n%#v\nexpected:\n%#v", got, expected)
	}
	t.Logf("got:\n%#v", got)
}

const stationMsg = `fa:ce:aa:bb:12:34
flags=[AUTH][ASSOC][AUTHORIZED][WMM][HT][VHT]
aid=6
capability=0x111
listen_interval=20
supported_rates=8c 12 98 24 b0 48 60 6c
timeout_next=NULLFUNC POLL
dot11RSNAStatsSTAAddress=fa:ce:aa:bb:12:34
dot11RSNAStatsVersion=1
dot11RSNAStatsSelectedPairwiseCipher=00-12-23-4
dot11RSNAStatsTKIPLocalMICFailures=0
dot11RSNAStatsTKIPRemoteMICFailures=0
wpa=2
AKMSuiteSelector=00-23-45-4
hostapdWPAPTKState=11
hostapdWPAPTKGroupState=0
rx_packets=91280
tx_packets=110847
rx_bytes=13716568000
tx_bytes=90628298
inactive_msec=590
signal=-64
rx_rate_info=60
tx_rate_info=3240 vhtmcs 8 vhtnss 2
rx_vht_mcs_map=fffa
tx_vht_mcs_map=fffa
ht_mcs_bitmask=ffff0000000000000000
last_ack_signal=-95
min_txpower=-7
max_txpower=21
vht_caps_info=0x0f817032
ht_caps_info=0x006f
ext_capab=0000000000000040
`
