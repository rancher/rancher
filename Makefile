TARGETS := $(filter-out container-run,$(shell ls scripts))
DEV_TARGETS := $(shell ls dev-scripts)

.DEFAULT_GOAL := ci

$(TARGETS):
	./scripts/container-run "$@"

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

.PHONY: $(TARGETS) $(DEV_TARGETS) quick-agent quick-server quick-binary-server quick-k3s-images
