# wifi-presence

OpenWrt package feed and build system for wifi-presence.

This is a special branch for use of `wifi-presence` with OpenWrt.
See the [`main`](https://github.com/awilliams/wifi-presence/tree/main) branch for the primary source code and documentation.

### Usage with OpenWrt [build system](https://openwrt.org/docs/guide-developer/toolchain/use-buildsystem)

```
# Add wifi-presence repository as a feed source.
# Note the ';openwrt' suffix which specifies use of the 'openwrt' branch.
echo "src-git awilliams https://github.com/awilliams/wifi-presence;openwrt" >> feeds.conf

# Update and install this package.
./scripts/feeds update awilliams
./scripts/feeds install wifi-presence
```

## Package Generation

Build `wifi-presence` OpenWrt packages (`.ipk`) for all supported architectures.

Requirements:
* Docker

Usage:

```shell
make packages
```

The resulting packages will be copied to the `./out` directory.
