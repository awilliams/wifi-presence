GO_TAG:=1.16

.PHONY: all
all: clean wifi-presence.ipk wifi-presence.linux.mips wifi-presence.darwin.amd64
	@ echo "Building all"

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: wifi-presence.ipk
wifi-presence.ipk:
	@./build.bash

.PHONY: wifi-presence.linux.mips
wifi-presence.linux.mips:
	docker run \
		--volume $(shell cd .. && pwd):/src:ro \
		--volume $(shell cd bin && pwd):/OUT \
		--workdir /src \
		--env GOARCH=mips \
		--env GOOS=linux \
		--env GOMIPS=softfloat \
		golang:${GO_TAG} \
			go build \
				-o /OUT/wifi-presence.linux.mips \
				-ldflags="-s -w" \
				-trimpath \
				./cmd/wifi-presence

.PHONY: wifi-presence.darwin.amd64
wifi-presence.darwin.amd64:
	docker run \
		--volume $(shell cd .. && pwd):/src:ro \
		--volume $(shell cd bin && pwd):/OUT \
		--workdir /src \
		--env GOARCH=amd64 \
		--env GOOS=darwin \
		golang:${GO_TAG} \
			go build \
				-o /OUT/wifi-presence.darwin.amd64 \
				-ldflags="-s -w" \
				-trimpath \
				./cmd/wifi-presence
