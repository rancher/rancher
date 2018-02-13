# Copyright 2015 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO    := GO15VENDOREXPERIMENT=1 go
PROMU := $(GOPATH)/bin/promu
pkgs   = $(shell $(GO) list ./... | grep -v -E '/vendor/|/ui')

PREFIX                  ?= $(shell pwd)
BIN_DIR                 ?= $(shell pwd)
FRONTEND_DIR            = $(BIN_DIR)/ui/app
DOCKER_IMAGE_NAME       ?= alertmanager
DOCKER_IMAGE_TAG        ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

ifdef DEBUG
	bindata_flags = -debug
endif


all: format build test

test:
	@echo ">> running tests"
	@$(GO) test -short $(pkgs)

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

# Will only build the back-end
build: promu
	@echo ">> building binaries"
	@$(PROMU) build --prefix $(PREFIX) $(BINARIES)

# Will build both the front-end as well as the back-end
build-all: assets build

tarball: promu
	@echo ">> building release tarball"
	@$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

assets: go-bindata ui/bindata.go template/internal/deftmpl/bindata.go

go-bindata:
	-@$(GO) get -u github.com/jteeuwen/go-bindata/...

template/internal/deftmpl/bindata.go: template/default.tmpl
	@go-bindata $(bindata_flags) -mode 420 -modtime 1 -pkg deftmpl -o template/internal/deftmpl/bindata.go template/default.tmpl

ui/bindata.go: ui/app/script.js ui/app/index.html ui/lib
# Using "-mode 420" and "-modtime 1" to make assets make target deterministic.
# It sets all file permissions and time stamps to 420 and 1
	@go-bindata $(bindata_flags) -mode 420 -modtime 1 -pkg ui -o \
		ui/bindata.go ui/app/script.js \
		ui/app/index.html \
		ui/app/favicon.ico \
		ui/lib/...

ui/app/script.js: $(shell find ui/app/src -iname *.elm)
	cd $(FRONTEND_DIR) && $(MAKE) script.js

promu:
	@GOOS=$(shell uname -s | tr A-Z a-z) \
	GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
	$(GO) get -u github.com/prometheus/promu

proto:
	scripts/genproto.sh

clean:
	rm template/internal/deftmpl/bindata.go
	rm ui/bindata.go
	cd $(FRONTEND_DIR) && $(MAKE) clean

.PHONY: all style format build test vet assets tarball docker promu proto
