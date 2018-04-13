default: build

GITHUB_REPO ?= github.com/heptio/authenticator
GORELEASER := $(shell command -v goreleaser 2> /dev/null)

.PHONY: build test format

build:
ifndef GORELEASER
	$(error "goreleaser not found (`go get -u -v github.com/goreleaser/goreleaser` to fix)")
endif
	$(GORELEASER) --skip-publish --rm-dist --snapshot

test:
	go test -v -cover -race $(GITHUB_REPO)/...

format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"
