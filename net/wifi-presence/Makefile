# https://openwrt.org/docs/guide-developer/packages

include $(TOPDIR)/rules.mk

PKG_NAME:=wifi-presence
PKG_VERSION:=0.0.3
PKG_RELEASE?=1
PKG_LICENSE:=MIT

PKG_SOURCE:=wifi-presence-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=https://codeload.github.com/awilliams/wifi-presence/tar.gz/v$(PKG_VERSION)?
PKG_HASH:=ec8ddc5c00764c4be4d90479fb2b088b18fffdc552b7a00db97241d258ae9481
PKG_BUILD_DIR:=$(BUILD_DIR)/wifi-presence-$(PKG_VERSION)

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_USE_MIPS16:=0

GO_PKG:=github.com/awilliams/wifi-presence
GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/wifi-presence

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/wifi-presence
  SECTION:=net
  CATEGORY:=Network
  TITLE:=WiFi presence detection
  URL:=https://github.com/awilliams/wifi-presence
  DEPENDS:=$(GO_ARCH_DEPENDS)
  MENU:=1
endef

# Package description; a more verbose description on what our package does
define Package/wifi-presence/description
WiFi presence detection. 
Publish WiFi client connect and disconnect events to MQTT.
endef

define Package/wifi-presence/conffiles
/etc/config/wifi-presence
endef

define Package/wifi-presence/postinst
#!/bin/sh
cat <<EOF
Please configure wifi-presence by editing the configuration
file: /etc/config/wifi-presence.
EOF
endef

# Package install instructions; create a directory inside the package to hold our executable,
# and then copy the executable we built previously into the folder
define Package/wifi-presence/install
	$(call GoPackage/Package/Install/Bin,$(PKG_INSTALL_DIR))

	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_INSTALL_DIR)/usr/bin/wifi-presence $(1)/usr/bin

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/wifi-presence.conf $(1)/etc/config/wifi-presence

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/wifi-presence.init $(1)/etc/init.d/wifi-presence
endef

$(eval $(call GoBinPackage,wifi-presence))
$(eval $(call BuildPackage,wifi-presence))
