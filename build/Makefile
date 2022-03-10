GO_TAG:=latest

.PHONY: all
all: clean wifi-presence.ipq806x.ipk wifi-presence.ath79.ipk
	@ echo "Building all"

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: wifi-presence.ipq806x.ipk
wifi-presence.ipq806x.ipk:
	@SDK=ipq806x-generic-19.07.7 ./build.bash

.PHONY: wifi-presence.ath79.ipk
wifi-presence.ath79.ipk:
	@SDK=ath79-generic-19.07.7 ./build.bash

.PHONY: shell
shell:
	docker run \
		--rm \
		-it \
		--workdir /home/build/openwrt \
		--volume $(shell cd .. && pwd):/SRC:ro \
		--volume $(shell cd bin && pwd):/OUT \
		wifi-presence/openwrt:ath79-generic-19.07.7

.PHONY: wifi-presence.linux.mips
wifi-presence.linux.mips:
	docker run \
		--volume $(shell cd .. && pwd):/src:ro \
		--volume $(shell pwd):/OUT \
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
