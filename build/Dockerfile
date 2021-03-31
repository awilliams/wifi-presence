ARG SDK=ath79-generic-19.07.7
FROM openwrtorg/sdk:${SDK}

# Git commit that corresponds to Go 1.16
ENV PACKAGES_SHA=dc587eccb7ad9f12ef4498ecb339a484299c39ba

# Build the Go compiler from the official packages source.
# Update the feeds file to point to a particular git commit
# of the 'feeds' source. This is to get the latest version of
# the Go compiler.
RUN \
	sed -i "s/\(\/packages.git\\^\).*/\1${PACKAGES_SHA}/" feeds.conf.default && \
	./scripts/feeds update packages && \
	./scripts/feeds install golang && \
	make defconfig && \
	make package/golang/compile
