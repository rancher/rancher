# Developing Rancher

## Generate a local container image

If you need to test some changes in a container image before they make it to a
pull request, you can use the handy `make quick` script.

This script uses `docker buildx` in order to enable cross-building of different
architecture images. To build an image for your current OS and architecture, run
from the Rancher project root:
```shell
TAG="localhost:5000/my-test-repo/image:tag" make quick
```

If you wish to cross-build for a different architecture, set the variable `ARCH`:
```shell
TAG="localhost:5000/my-test-repo/image:tag" \
  ARCH="amd64" \
  make quick
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
