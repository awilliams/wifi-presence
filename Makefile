OPENWRT_VERSION=21.02.2
IMG_TAG:=wifi-presence/openwrt:${OPENWRT_VERSION}

.PHONY: packages
packages: openwrt-${OPENWRT_VERSION}
	mkdir -p out
	docker run \
		--rm \
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
