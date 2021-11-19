# wifi-presence ![CI](https://github.com/awilliams/wifi-presence/workflows/CI/badge.svg?branch=main) [![Go Reference](https://pkg.go.dev/badge/github.com/awilliams/wifi-presence.svg)](https://pkg.go.dev/github.com/awilliams/wifi-presence)

Presence detection based on WiFi connections to APs (access points).
Client connect and disconnect events are published to MQTT.

* **What**: Standalone application designed to run on WiFi routers. Monitors WiFi client connect and disconnect events and publishes them to an MQTT broker.
* **Why**: Presence detection for home automation systems.
* **How**: `wifi-presence` connects to [`hostapd`'s control interface](http://w1.fi/wpa_supplicant/devel/hostapd_ctrl_iface_page.html) to receive client connect and disconnect events.

## OpenWRT

This branch provides a package feed for OpenWRT.

### Usage

```
# Add wifi-presence repository as a feed source.
# Note the ';openwrt' suffix which specifies use of the 'openwrt' branch.
echo "src-git awilliams https://github.com/awilliams/wifi-presence;openwrt" >> feeds.conf

# Update and install this package.
./scripts/feeds update awilliams
./scripts/feeds install wifi-presence
```
