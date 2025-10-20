TARGETS := $(shell ls scripts)
DEV_TARGETS := $(shell ls dev-scripts)

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
