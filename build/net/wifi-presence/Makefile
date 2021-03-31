# https://openwrt.org/docs/guide-developer/packages

include $(TOPDIR)/rules.mk

PKG_NAME:=wifi-presence
PKG_VERSION?=DEV
PKG_RELEASE?=1
PKG_LICENSE:=MIT

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_USE_MIPS16:=0

GO_PKG:=github.com/awilliams/wifi-presence
GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/wifi-presence

# Source settings (i.e. where to find the source codes)
# This is a custom variable, used below
SOURCE_DIR:=/SRC

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

# Package definition; instructs on how and where our package will appear in the overall configuration menu ('make menuconfig')
define Package/wifi-presence
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Publishes WiFi events to MQTT for presence detection
  URL:=https://github.com/awilliams/wifi-presence
endef

# Package description; a more verbose description on what our package does
define Package/wifi-presence/description
WiFi presence detection tool
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

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	cp -r $(SOURCE_DIR)/* $(PKG_BUILD_DIR)
	$(Build/Patch)
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
