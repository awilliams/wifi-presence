OPENWRT_VERSION=22.03.0
IMG_TAG:=wifi-presence/openwrt:${OPENWRT_VERSION}

# Create wifi-presence ipk for all supported architectures.
.PHONY: packages
packages: openwrt-${OPENWRT_VERSION}
	mkdir -p out
	docker run \
		--rm \
		--volume $(shell pwd)/keys:/keys:ro \
		--volume $(shell pwd)/package.bash:/package.bash:ro \
		--volume $(shell pwd)/out:/OUT \
		${IMG_TAG} \
			/package.bash

# Start interactive shell container from OpenWRT SDK image.
.PHONY: shell
shell: openwrt-${OPENWRT_VERSION}
	mkdir -p out
	docker run \
		--rm \
		-it \
		--volume $(shell pwd)/keys:/keys:ro \
		--volume $(shell pwd)/package.bash:/package.bash:ro \
		--volume $(shell pwd)/out:/OUT \
		${IMG_TAG}

# Create the OpenWrt SDK image.
.PHONY: openwrt-${OPENWRT_VERSION}
openwrt-${OPENWRT_VERSION}:
	docker build \
		--build-arg OPENWRT_VERSION=${OPENWRT_VERSION} \
		--tag ${IMG_TAG} \
		- < Dockerfile
