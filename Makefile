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

scale-cluster-agent:
	@echo "Building scale-cluster-agent..."
	@mkdir -p bin
	@cd scale-cluster-agent && go mod tidy
	@cd scale-cluster-agent && go build -o ../bin/scale-cluster-agent main.go
	@echo "Scale cluster agent built successfully: bin/scale-cluster-agent"

$(DEV_TARGETS):
	./dev-scripts/$@

.PHONY: $(TARGETS) $(DEV_TARGETS) quick-agent quick-server scale-cluster-agent
