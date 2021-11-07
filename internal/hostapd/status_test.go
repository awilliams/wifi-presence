package hostapd

import "testing"

func TestStatusParse(t *testing.T) {
	var got Status
	if err := got.parse([]byte(statusMsg)); err != nil {
		t.Fatal(err)
	}

	expected := Status{
		State:      "ENABLED",
		Channel:    52,
		MaxTxPower: 23,
		SSID:       "🌝",
		BSSID:      "aa:bb:cc:ee:12:34",
	}
	if got != expected {
		t.Fatalf("got:\n%#v\nexpected:\n%#v", got, expected)
	}
	t.Logf("got:\n%#v", got)
}

const statusMsg = `state=ENABLED
phy=phy0
freq=5260
num_sta_non_erp=0
num_sta_no_short_slot_time=5
num_sta_no_short_preamble=5
olbc=0
num_sta_ht_no_gf=5
num_sta_no_ht=0
num_sta_ht_20_mhz=1
num_sta_ht40_intolerant=0
olbc_ht=0
ht_op_mode=0x6
cac_time_seconds=60
cac_time_left_seconds=N/A
channel=52
secondary_channel=1
ieee80211n=1
ieee80211ac=1
ieee80211ax=0
beacon_int=100
dtim_period=2
vht_oper_chwidth=0
vht_oper_centr_freq_seg0_idx=54
vht_oper_centr_freq_seg1_idx=0
vht_caps_info=338001b2
rx_vht_mcs_map=fffa
tx_vht_mcs_map=fffa
ht_caps_info=09ef
ht_mcs_bitmask=ffff0000000000000000
supported_rates=0c 12 18 24 30 48 60 6c
max_txpower=23
bss[0]=wlan0
bssid[0]=aa:bb:cc:ee:12:34
ssid[0]=\xf0\x9f\x8c\x9d
num_sta[0]=5
chan_util_avg=96`
