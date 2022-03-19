# Signing Keys

Storage location of public/private `usign` keys.

See https://openwrt.org/docs/guide-user/security/keygen#generate_usign_key_pair

*Create Keys*

From within OpenWrt Docker container:

```shell
usign -G -c "OpenWrt usign key of <name>" \
  -s private.key -p public.key
```
