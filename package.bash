#!/usr/bin/env bash

# Create wifi-presence OpenWrt packages (.ipk) for
# all supported architectures.
#
# This script is written to be run within the OpenWrt SDK Docker container.
# See Makefile for more information.

set -e

if [ ! -f feeds.conf ] || grep -q -v "wifi-presence" feeds.conf; then
  # Add wifi-presence repository as a feed source.
  echo "src-git awilliams https://github.com/awilliams/wifi-presence;openwrt" >> feeds.conf
  # Update and install this package.
  ./scripts/feeds update awilliams
  ./scripts/feeds install wifi-presence
fi

version=$(grep "^PKG_VERSION" feeds/awilliams/net/wifi-presence/Makefile | sed -E 's/PKG_VERSION:=//')
release=$(grep "^PKG_RELEASE" feeds/awilliams/net/wifi-presence/Makefile | sed -E 's/PKG_RELEASE[\?:]=//')

echo "Packaging wifi-presence v${version}-${release}"

# Architectures that are not supported by Go, in 'egrep' format.
unsupported_archs="^(arc_.*|powerpc_.*)$"

archs=($(./scripts/dump-target-info.pl architectures 2>/dev/null | awk '{ print $1 }'))
targets="$(./scripts/dump-target-info.pl targets 2>/dev/null)"

for arch in "${archs[@]}"; do
  # Find first target using 'arch'.
  target=$(echo "${targets}" | grep "\b${arch}\b" | head -n 1 | awk '{ print $1 }')
  # Split target using '/' delimiter. Expect 2 parts.
  readarray -d "/" -t target_parts < <(printf '%s' "${target}")

  if echo "${arch}" | egrep -q "${unsupported_archs}"; then
    echo "Arch '${arch}' is not supported by Go, skipping."
    continue
  fi

  pkgName="wifi-presence_${version}-${release}_${arch}.ipk"

  ipk=$(find /OUT/ -iname "${pkgName}" -type f)
  if [[ -n "${ipk}" ]]; then
    echo "Package already exists (${ipk}), skipping."
    continue
  fi

  echo "## Building for:\t${arch}"
  echo "## Using target:\t${target_parts[0]}/${target_parts[1]}"

  make clean
  make toolchain/clean
  rm -f .config

  # This seems to be the minimal required configuration
  # in order for `make defconfig` to then generate the proper
  # configuration for this architecture.
  echo "CONFIG_TARGET_${target_parts[0]}=y" >> .config
  echo "CONFIG_TARGET_${target_parts[0]}_${target_parts[1]}=y" >> .config
  make defconfig

  # Set the location of the Go toolchain.
  # See: https://github.com/openwrt/packages/issues/12793
  # See: Dockerfile
  echo "CONFIG_GOLANG_EXTERNAL_BOOTSTRAP_ROOT=\"/usr/local/go\"" >> .config
  # Enable wifi-presence.
  echo "CONFIG_PACKAGE_wifi-presence=y" >> .config

  # Build toolchain.
  make -j $(nproc) tools/install
  make -j $(nproc) toolchain/install

  # Build wifi-presence package.
  make -j $(nproc) package/wifi-presence/compile

  # Copy package to output directory.
  find "bin/packages/${arch}" -iname "${pkgName}" -type f -exec cp "{}" /OUT/. \;
done
