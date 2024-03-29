# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Changed
- Update `go.mod` from Go 1.16 to Go 1.19
- Update [eclipse/paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) library from `v1.3.5` to `v1.4.2`

## [v0.3.0] - 2022-11-20
### Fixed
- Proper handling of new hostapd `AP-STA-CONNECTED` messages (#13)

## [v0.2.0] - 2022-09-12
### Fixed
- Proper decoding of SSID names which contain non-ascii or control characters (#8)

## [v0.1.2] - 2022-03-21
### Changed
- Improved README
- Update the `hostapd-ctrl` program to emulate a 'mini' version of hostapd
- Address go lint and spelling issues

## [v0.1.1] - 2022-03-11
### Added
- Adds support for hostapd-mini (non-full version). When using this version of hostapd, wifi-presence will consider all devices as disconnected at startup.

### Changed
- Change some JSON attribute field names to match Home Assistant usage.
- Do not update state when config changes name only.

## [v0.1.0] - 2022-02-04
### Added
- Adds support for [Home Assistant MQTT Discovery](https://www.home-assistant.io/integrations/device_tracker.mqtt/)

### Changed
- Changes MQTT topic structure

## [v0.0.2] - 2021-02-25
### Added
- Add `-version` flag

### Changed
- Handle case where client transitions to different SSID on same AP
- Build with Go 1.16

## [v0.0.1] - 2021-02-15
Initial beta release

[Unreleased]: https://github.com/awilliams/wifi-presence/compare/v0.2.0...HEAD
[v0.2.0]: https://github.com/awilliams/wifi-presence/compare/v0.1.2...v0.2.0
[v0.1.2]: https://github.com/awilliams/wifi-presence/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/awilliams/wifi-presence/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/awilliams/wifi-presence/compare/v0.0.2...v0.1.0
[v0.0.2]: https://github.com/awilliams/wifi-presence/compare/v0.0.1...v0.0.2
[v0.0.1]: https://github.com/awilliams/wifi-presence/releases/tag/v0.0.1
