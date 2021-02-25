FROM openwrtorg/sdk:ath79-generic-19.07.5

# Git commit that corresponds to Go 1.16
ENV PACKAGES_SHA=dc587eccb7ad9f12ef4498ecb339a484299c39ba

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
