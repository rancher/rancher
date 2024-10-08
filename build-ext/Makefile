BUILD := docker buildx build

IMAGE_TAG ?= dev
REGISTRY ?= local
REGISTRY_IMAGE ?= $(REGISTRY)/api-extension

K3D_CLUSTER ?= imperative-lo
K3D_K3S_VERSION ?= v1.30.2
K3D_VERSION ?= v5.7.2

DEEPCOPY_GEN ?= deepcopy-gen
OPENAPI_GEN ?= openapi-gen

all: image

k3d:
	mkdir -p bin
	@$(BUILD) \
		--target=k3d \
		--build-arg=K3D_VERSION=$(K3D_VERSION) \
		--output=type=local,dest=bin \
		.

create-cluster: k3d
	@k3d cluster create --image rancher/k3s:$(K3D_K3S_VERSION)-k3s1 $(K3D_CLUSTER)

delete-cluster: k3d
	@k3d cluster delete $(K3D_CLUSTER)

build:
	 CGO_ENABLED=0 go build -o ext ../cmd/ext/

image:
	@$(BUILD) \
		--target=apiserver-poc \
		--output=type=image,name=$(REGISTRY_IMAGE):$(IMAGE_TAG) \
		.

push:
	docker push $(REGISTRY_IMAGE):$(IMAGE_TAG)

import:
	@k3d image import --cluster $(K3D_CLUSTER) $(REGISTRY_IMAGE):$(IMAGE_TAG)

deploy:
	@$(MAKE) image import
	@kubectl apply -f deployment.yaml
	@kubectl -n cattle-system delete pods -l app=api-extension

deploy-external:
	@$(MAKE) image push
	@kubectl apply -f deployment.yaml
	@kubectl -n cattle-system delete pods -l app=api-extension

log:
	@kubectl -n cattle-system logs --tail=-1 -l app=api-extension -f

exec:
	@kubectl -n cattle-system exec -it $(shell kubectl -n cattle-system get pods -l app=api-extension -o json | jq '.items[].metadata.name' -r) -- bash

deepcopy-gen:
	@$(DEEPCOPY_GEN) --go-header-file ../scripts/boilerplate.go.txt ../pkg/ext/resources/tokens/
	@$(DEEPCOPY_GEN) --go-header-file ../scripts/boilerplate.go.txt ../pkg/ext/resources/useractivity/

openapi-gen:
	@$(OPENAPI_GEN) --output-pkg github.com/rancher/rancher/pkg/ext/generated/openapi/tokens --output-dir ../pkg/ext/generated/openapi/tokens/ --go-header-file ../scripts/boilerplate.go.txt  ../pkg/ext/resources/tokens/ k8s.io/apimachinery/pkg/apis/meta/v1
	@$(OPENAPI_GEN) --output-pkg github.com/rancher/rancher/pkg/ext/generated/openapi/useractivity --output-dir ../pkg/ext/generated/openapi/useractivity/ --go-header-file ../scripts/boilerplate.go.txt  ../pkg/ext/resources/useractivity/ k8s.io/apimachinery/pkg/apis/meta/v1

gen:
	@$(MAKE) deepcopy-gen
	@$(MAKE) openapi-gen

.PHONY: all k3d image import restart log push deploy-external deploy gen deepcopy-gen openapi-gen
