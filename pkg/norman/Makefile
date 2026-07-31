# Set default ARCH, but allow it to be overridden
UNAME_ARCH := $(shell uname -m)
ifeq ($(UNAME_ARCH),x86_64)
    ARCH ?= amd64
else ifeq ($(UNAME_ARCH),aarch64)
    ARCH ?= arm64
else
    ARCH ?= $(UNAME_ARCH)
endif

# Export ARCH so it's available to subshells
export ARCH

GOLANGCI_LINT_VERSION := v1.64.8
BIN_DIR := $(shell pwd)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

.PHONY: ci build test validate clean

ci: validate test build

build:
	@echo "--- Building Binary ---"
	@mkdir -p bin
	@if [ -n "$$(git status --porcelain --untracked-files=no)" ]; then DIRTY="-dirty"; fi; \
	COMMIT=$$(git rev-parse --short HEAD); \
	GIT_TAG=$$(git tag -l --contains HEAD | head -n 1); \
	if [ -z "$$DIRTY" ] && [ -n "$$GIT_TAG" ]; then VERSION=$$GIT_TAG; else VERSION="$$COMMIT$$DIRTY"; fi; \
	if [ "$$(uname)" != "Darwin" ]; then LINKFLAGS="-extldflags -static -s"; fi; \
	CGO_ENABLED=0 go build -ldflags "-X main.VERSION=$$VERSION $$LINKFLAGS" -o bin/norman ./example

test:
	@echo "--- Running Unit Tests ---"
	@PACKAGES=$$(find . -name '*.go' | xargs -I{} dirname {} |  cut -f2 -d/ | sort -u | grep -Ev '(^\.$$|.git|.trash-cache|vendor|bin)' | sed -e 's!^!./!' -e 's!$$!/...!'); \
	if [ "$$ARCH" = "amd64" ]; then RACE="-race"; fi; \
	CGO_ENABLED=1 go test $$RACE -cover -tags=test $$PACKAGES

$(GOLANGCI_LINT):
	@echo "--- Installing golangci-lint $(GOLANGCI_LINT_VERSION) ---"
	@mkdir -p $(BIN_DIR)
	@GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

validate: $(GOLANGCI_LINT)
	@echo "--- Validating ---"
	@export "GOROOT=$$(go env GOROOT)"; \
	$(GOLANGCI_LINT) run --timeout=5m

clean:
	@rm -rf bin dist
