# Adapated from https://github.com/mwarning/docker-openwrt-builder
FROM debian:buster

ARG OPENWRT_VERSION=21.02.2
ARG GO_BOOTSTRAP_VERSION=1.17.3

RUN apt-get update && \
  apt-get install -y \
    sudo time git-core subversion build-essential g++ bash make \
    libssl-dev patch libncurses5 libncurses5-dev zlib1g-dev gawk \
    flex gettext wget unzip xz-utils python python-distutils-extra \
    python3 python3-distutils-extra rsync curl libsnmp-dev liblzma-dev \
    libpam0g-dev cpio rsync vim && \
  apt-get clean && \
  useradd -m user && \
  echo 'user ALL=NOPASSWD: ALL' > /etc/sudoers.d/user

# Add local version of Go, since bootstrapping method normally used
# doesn't work on linux/arm version (Apple M1).
# https://github.com/openwrt/packages/issues/12793
RUN \
  mkdir /openwrt && \
  chown user:user openwrt && \
  wget https://golang.org/dl/go${GO_BOOTSTRAP_VERSION}.linux-arm64.tar.gz -O /tmp/go.tar.gz && \
  tar -C /usr/local -xzf /tmp/go.tar.gz && \
  rm -rf /tmp/go.tar.gz

USER user
WORKDIR /

# Set dummy git config.
RUN \
  git config --global user.name "user" && \
  git config --global user.email "user@example.com"

# Checkout OpenWrt and update feeds.
RUN \
  git clone https://git.openwrt.org/openwrt/openwrt.git && \
  cd openwrt && \
  git fetch --tags && \
  git checkout v${OPENWRT_VERSION} && \
  ./scripts/feeds update -a && \
  ./scripts/feeds install -a

WORKDIR /openwrt

# Compile tools and toolchain to save time on repeated builds.
RUN \
  make -j $(nproc) defconfig download tools/install
