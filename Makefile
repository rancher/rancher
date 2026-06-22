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

# Builds the integration setup binary without starting a container.
integration-setup:
	cd tests/v2/integration && ./scripts/build-integration-setup
	CATTLE_TEST_CONFIG=$(PWD)/tests/v2/integration/config.yaml \
	  ./tests/v2/integration/bin/integrationsetup

# Runs integration tests against an already-running Rancher server.
# Requires CATTLE_TEST_CONFIG to point at a valid config.yaml, or run
# 'make integration-setup' first to generate one automatically.
integration-test-local:
	CATTLE_TEST_CONFIG=$(PWD)/tests/v2/integration/config.yaml \
	  CGO_ENABLED=0 go test -v -failfast -timeout 30m -p 1 ./tests/v2/integration/...

$(DEV_TARGETS):
	./dev-scripts/$@

.PHONY: $(TARGETS) $(DEV_TARGETS) quick-agent quick-server quick-binary-server quick-k3s-images integration-setup integration-test-local
