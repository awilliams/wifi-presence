.PHONY:
test:
	go test -race ../...
	go test -race  -mqttAddr "tcp://localhost:1883" ../internal/hass ../internal/presence .
