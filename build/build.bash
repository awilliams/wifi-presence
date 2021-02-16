#!/usr/bin/env bash

# Build and package wifi-presence for OpenWRT.
# Uses the official openwrt/sdk Docker image. This image
# comes with the OpenWRT SDK and contains an OpenWRT specific
# Go toolchain.
#
# This script is executed from the host machine, and then
# again from within the started Docker container.

set -e

IMG="wifi-presence/openwrt:latest"

if [[ -z "${IS_WITHIN_DOCKER}" ]]; then
	# We are running on host, not within Docker.

	if ! docker info > /dev/null 2>&1; then
		echo "Error: Docker must be running."
		exit 1
	fi

	# Define build directory.
	DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

	# Build base image.
	docker build \
		-t ${IMG} \
		- < "${DIR}/Dockerfile"

	# Clear build directory.
	rm -rf "${DIR}/bin" && mkdir "${DIR}/bin"

	# Run this same script, but from within
	# the build container.
	docker run \
		--rm \
		--env IS_WITHIN_DOCKER=true \
		--workdir /home/build/openwrt \
		--volume "${DIR}/..":/SRC:ro \
		--volume "${DIR}/bin":/OUT \
		${IMG} \
		/SRC/build/build.bash

	# List artifacts.
	find bin -iname '*.ipk'

	exit 0
fi

# Now executing from within Docker container.

# Add this directory as a feed source.
echo "src-link local /SRC/build" >> feeds.conf

# Update this feed.
./scripts/feeds update local
./scripts/feeds install -a -p local

# Create default .config file.
make defconfig
# Enable build of our package.
sed -i \
	's/CONFIG_PACKAGE_wifi-presence=m/CONFIG_PACKAGE_wifi-presence=y/g' \
	.config

# Compile.
make package/wifi-presence/compile

# Move resulting package to output directory.
find ./bin/packages -name 'wifi-presence*.ipk' -exec mv -t /OUT/ {} +
