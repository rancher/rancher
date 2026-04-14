include hack/make/deps.mk

# Build defaults
REPO ?= rancher
TAG ?= dev
ARCH ?= amd64
OS ?= linux
COMMIT ?= $(shell $(CURDIR)/scripts/version | grep 'COMMIT:' | cut -d' ' -f2)

# Common build arguments
COMMON_ARGS += --build-arg VERSION=$(TAG)
COMMON_ARGS += --build-arg ARCH=$(ARCH)
COMMON_ARGS += --build-arg IMAGE_REPO=$(REPO)
COMMON_ARGS += --build-arg COMMIT=$(COMMIT)
COMMON_ARGS += $(DEPS_BUILD_ARGS)

# Platform
PLATFORM ?= $(OS)/$(ARCH)

# Build command
BUILD := docker buildx build

.PHONY: target-%
target-%: ## Builds the specified target defined in the Dockerfile.
	$(BUILD) \
		--target=$* \
		--platform=$(PLATFORM) \
		$(COMMON_ARGS) \
		$(TARGET_ARGS) \
		--file ./package/Dockerfile .

TARGETS := $(shell ls scripts)
DEV_TARGETS := $(shell ls dev-scripts)

include hack/make/deps.mk
export DEPS_BUILD_ARGS

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

$(TARGETS): .dapper
	@if [ "$@" = "check-chart-kdm-source-values" ]; then \
		./.dapper -q --no-out $@; \
	else \
		./.dapper $@; \
	fi

.DEFAULT_GOAL := ci

quick-agent:
	@$(MAKE) quick TARGET="agent"

quick-server:
	@$(MAKE) quick TARGET="server"

quick-binary-server:
	@$(MAKE) quick TARGET="binary-server"

quick-k3s-images:
	@$(MAKE) quick TARGET="k3s-images"

$(DEV_TARGETS):
	./dev-scripts/$@

.PHONY: $(TARGETS) $(DEV_TARGETS) quick-agent quick-server quick-binary-server
