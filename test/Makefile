.PHONY: test
test:
	go test \
		-race \
		../...
	go test \
		. \
		../internal/presence \
		../internal/hass \
		-race \
		-v \
		-mqttAddr "tcp://localhost:1883"

.PHONY: wifi-presence
wifi-presence:
	go run ../cmd/wifi-presence \
		-apName="test-AP" \
		-debounce="2s" \
		-hass.autodiscovery=true \
		-mqtt.addr="tcp://localhost:1883" \
		-hostapd.socks="./hostapd.sock" \
		-verbose

.PHONY: hostap-ctrl
hostap-ctrl:
	go run ./hostap-ctrl

.PHONY: send-cfg
send-cfg:
	echo '{"devices":[{"name":"test-phone","mac":"BE:EF:00:00:FA:CE"}]}' \
	| mosquitto_pub \
		-h 'localhost' \
		-t 'wifi-presence/config' \
		-l

.PHONY: send-empty-cfg
send-empty-cfg:
	echo '{"devices":[]}' \
	| mosquitto_pub \
		-h 'localhost' \
		-t 'wifi-presence/config' \
		-l
