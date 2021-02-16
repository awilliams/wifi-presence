FROM openwrtorg/sdk:ath79-generic-19.07.5

# Git commit that corresponds to Go 1.15.6
ENV PACKAGES_SHA=b1db2761b2562309c8f64e2fb3612333a6b144fe

# Reference only the Go package from the official
# openwrt/packages repo.
RUN \
	cd ${HOME} && \
	mkdir packages && \
	cd packages && \
	git init && \
	git remote add origin https://git.openwrt.org/feed/packages.git && \
	git fetch origin ${PACKAGES_SHA} && \
	git reset --hard FETCH_HEAD && \
	cd ${HOME}/openwrt && \
	echo "src-link packages /home/build/packages/lang/golang" >> feeds.conf && \
	./scripts/feeds update -a && \
	./scripts/feeds install -a && \
	make defconfig && \
	make package/golang/compile
