# Developing Rancher

## Generate a local container image

If you need to test some changes in a container image before they make it to a
pull request, you can use the handy [build-local.sh](../dev-scripts/build-local.sh)
script.

This script uses `docker buildx` in order to enable cross-building of different
architecture images. To build an image for your current OS and architecture, run
from the Rancher project root:
```shell
TARGET_REPO="localhost:5000/my-test-repo/image:tag" dev-scripts/build-local.sh
```

If you wish to cross-build for a different OS or architecture, set the variables
`TARGET_ARCH` and/or `TARGET_OS`:
```shell
TARGET_REPO="localhost:5000/my-test-repo/image:tag" \
  TARGET_ARCH="amd64" \
  TARGET_OS="linux" \
  dev-scripts/build-local.sh
```

To specify a `go` binary other than the one present in your `PATH`, use the
`GO_BINARY` environment variable:
```shell
GO_BINARY="/opt/go1.18/bin/go" dev-scripts/build-local.sh
```

## Deploy your custom image via Helm

To deploy a custom image via Helm, set the variables `rancherImage` and `rancherImageTag`:
```shell
helm upgrade --install rancher/rancher \
  --namespace cattle-system \
  --create-namespace \
  --set rancherImage="my-test-repo/image" \
  --set rancherImageTag="dev-tag"
```