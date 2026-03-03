TARGETS := $(shell ls scripts)
DEV_TARGETS := $(shell ls dev-scripts)
MACHINE ?= "rancher"

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

buildx-machine:
	@docker buildx ls | grep $(MACHINE) || \
	docker buildx create --name=$(MACHINE) --driver=docker-container

quick-agent:
	@$(MAKE) quick TARGET="agent"

quick-server:
	@$(MAKE) quick TARGET="server"

quick-binary-server:
	@$(MAKE) quick TARGET="binary-server"

quick-k3s-images:
	@$(MAKE) quick TARGET="k3s-images"

build-rancher-server: buildx-machine
	@$(MAKE) BUILDER="$(MACHINE)" build TARGET="build-server"

build-rancher-server-tarball: buildx-machine
	@$(MAKE) BUILDER="$(MACHINE)" build TARGET="build-server-tarball"

build-rancher-agent: buildx-machine
	@$(MAKE) BUILDER="$(MACHINE)" build TARGET="build-agent"
build-rancher-agent-tarball: buildx-machine
	@$(MAKE) BUILDER="$(MACHINE)" build TARGET="build-agent-tarball"

$(DEV_TARGETS):
	./dev-scripts/$@

.PHONY: $(TARGETS) $(DEV_TARGETS) quick-agent quick-server quick-binary-server build-rancher-server build-rancher-server-tarball build-rancher-agent build-rancher-agent-tarball
