TINI_VERSION := v0.18.0
TINI_URL_amd64 := https://github.com/krallin/tini/releases/download/$(TINI_VERSION)/tini
TINI_URL_arm64 := https://github.com/krallin/tini/releases/download/$(TINI_VERSION)/tini-arm64
TINI_URL_s390x := https://github.com/krallin/tini/releases/download/$(TINI_VERSION)/tini-s390x
TINI_HASH_amd64 := 12d20136605531b09a2c2dac02ccee85e1b874eb322ef6baf7561cd93f93c855
TINI_HASH_arm64 := 7c5463f55393985ee22357d976758aaaecd08defb3c5294d353732018169b019
TINI_HASH_s390x := c8aaa618ea7897f26979ea10920373e06f3e6dfeb41ef95342eda2eb5672f24d

DEPS_BUILD_ARGS := --build-arg TINI_VERSION=$(TINI_VERSION) \
	--build-arg TINI_URL_amd64=$(TINI_URL_amd64) \
	--build-arg TINI_URL_arm64=$(TINI_URL_arm64) \
	--build-arg TINI_URL_s390x=$(TINI_URL_s390x) \
	--build-arg TINI_HASH_amd64=$(TINI_HASH_amd64) \
	--build-arg TINI_HASH_arm64=$(TINI_HASH_arm64) \
	--build-arg TINI_HASH_s390x=$(TINI_HASH_s390x)
