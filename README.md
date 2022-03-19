# wifi-presence

OpenWrt package feed and build system for wifi-presence.

This is a special branch for use of `wifi-presence` with OpenWrt.
See the [`main`](https://github.com/awilliams/wifi-presence/tree/main) branch for the primary source code and documentation.

### Usage with opkg

```shell
# Add public key
wget https://wifi-presence.s3.us-east-2.amazonaws.com/public.key
opkg-key add public.key

# Add package source as a custom feed
echo "src/gz wifi-presence http://wifi-presence.s3-website.us-east-2.amazonaws.com" >> /etc/opkg/customfeeds.conf
```

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
