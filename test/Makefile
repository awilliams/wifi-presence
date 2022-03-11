.PHONY:
test:
	go test -race ../...
	go test . ../internal/presence ../internal/hass -race -v -mqttAddr "tcp://localhost:1883"
