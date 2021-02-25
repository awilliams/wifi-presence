# wifi-presence ![CI](https://github.com/awilliams/wifi-presence/workflows/CI/badge.svg?branch=main) [![Go Reference](https://pkg.go.dev/badge/github.com/awilliams/wifi-presence.svg)](https://pkg.go.dev/github.com/awilliams/wifi-presence)

Presence detection based on WiFi connections to APs (access points).
Client connect and disconnect events are published to MQTT.

* **What**: Standalone application designed to run on WiFi routers. Monitors WiFi client connect and disconnect events and publishes them to an MQTT broker.
* **Why**: Presence detection for home automation systems.
* **How**: `wifi-presence` connects to [`hostapd`'s control interface](http://w1.fi/wpa_supplicant/devel/hostapd_ctrl_iface_page.html) to receive client connect and disconnect events.

OpenWrt Requirements:
 * [This commit](https://github.com/openwrt/openwrt/commit/1ccf4bb93b0304c3c32a8a31a711a6ab889fd47a)

This program was designed for OpenWrt APs, but should work on any system meeting the following requirements:
 * Running [hostapd](http://w1.fi/hostapd/)
 * Linux operating system with supported architecture

## Motivation

A home automation system that reacts to presence and absence events provides a more automated experience.

There are many ways to determine presence, e.g. motion sensors, network traffic monitoring, GPS & geo-fencing, etc.
A person's phone typically travels with them in and out of a household.
Most phones automatically connect and disconnect from home WiFi networks.
Therefore a WiFi connection to one or more access points (APs) can be used as a proxy for physical presence.

There are similar projects that periodically ping client devices.
This method may be less reliable than using `hostapd` because phones may not respond to pings while in low power mode.
There is also a delay introduced by the ping frequency.

## Usage

```
$ wifi-presence -h
Usage: wifi-presence [options]

Options:
  -apName string
    	Access point name (default "hostname")
  -debounce duration
    	Time to wait until considering a station disconnected. Examples: 5s, 1m (default 10s)
  -hostapd.socks string
    	Hostapd control interface socket(s). Separate multiple paths by ':'
  -mqtt.addr string
    	MQTT broker address, e.g "tcp://mqtt.broker:1883"
  -mqtt.id string
    	MQTT client ID (default "wifi-presence.hostname")
  -mqtt.password string
    	MQTT password (optional)
  -mqtt.prefix string
    	MQTT topic prefix (default "wifi-presence")
  -mqtt.username string
    	MQTT username (optional)
  -v	Verbose logging (alias)
  -verbose
    	Verbose logging
  -version
    	Print version and exit

About:
wifi-presence monitors a WiFi access point (AP) and publishes events to an MQTT topic
when clients connect or disconnect. The debounce option can be used to delay sending
a disconnect event. This is useful to prevent events from clients that quickly
disconnect then re-connect.

hostapd/wpa_supplicant:
wifi-presence requires hostapd running with control interface(s) enabled.
The hostapd option is 'ctrl_interface'. More information:
https://w1.fi/cgit/hostap/plain/hostapd/hostapd.conf

The -hostapd.socks option should correspond to the socket
locations defined by 'ctrl_interface'. Multiple sockets
can be monitored (one socket per radio is created by hostapd).

MQTT:
The prefix of the MQTT topic is configurable using
options defined above:

$mqtt.prefix/$apName/$clientMAC

The body of the connect disconnect messages is JSON. Example:

{
  "ap": "MyRouter",
  "ssid": "wifi-name",
  "bssid": "XX:XX:XX:XX:XX:XX",
  "mac": "ab:cd:ef:12:34:56",
  "action": "connect",
  "timestamp": "2021-02-25T10:16:31.455852-07:00"
}

The program will publish a status message when starting and exiting
to the following topic:

$mqtt.prefix/$apName/status

The body of the message is JSON. Example:

{
  "status": "online",
  "timestamp": "2021-02-25T10:16:31.45609-07:00"
}
```

## OpenWrt

The [OpenWrt](https://openwrt.org/about) project is
> OpenWrt is a highly extensible GNU/Linux distribution for embedded devices (typically wireless routers).
There are OpenWrt compatible packages of `wifi-presence` available for download.

See the [build](./build) directory for more information.

## Go

The `github.com/awilliams/wifi-presence/pkg/presence` Go package provides types for consuming `wifi-presence` messages.

## iOS

iOS version 14 introduced ["private Wi-Fi addresses"](https://support.apple.com/en-us/HT211227) to improve privacy.
When enabled, an iOS client will connect to APs using different MAC addresses. Consider disabling this feature for APs that
you control and are running `wifi-presence` to help make presence detection configuration easier.
