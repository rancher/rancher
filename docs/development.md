# Developing Rancher

## Generate a local container image

If you need to test some changes in a container image before they make it to a
pull request, you can use the handy `make quick` script.

This script uses `docker buildx` in order to enable cross-building of different
architecture images. To build an image for your current OS and architecture, run
from the Rancher project root:
```shell
REPO="localhost:5000/my-test-repo/image" TAG="tag" make quick
```

If you wish to cross-build for a different architecture, set the variable `ARCH`:
```shell
REPO="localhost:5000/my-test-repo/image" \
  TAG="tag" \
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

## Local dev flow for the CAPRKE2 v2prov test

`Test_Operation_SetE_CAPRKE2DockerOperations` is a local-only integration test that exercises
Rancher's operation adapters (etcd snapshot save/restore, encryption-key rotation) against a
CAPI Cluster whose control plane is a CAPRKE2 `RKE2ControlPlane` on the CAPI Docker
infrastructure provider. CI does not run it — the required setup (kind docker network + host
docker socket exposed to the mgmt cluster's node) is only wired up locally.

Prerequisites on your machine: `docker`, `k3d`, `kubectl`.

1. **Provision the local cluster:**
   ```shell
   make dev-env
   ```
   Creates the `kind` docker network (which CAPD hardcodes) and a k3d cluster (`local-caprke2`
   by default) attached to it, with `/var/run/docker.sock` bind-mounted into the server node.
   Points your default kubectl context at the new cluster. Idempotent.

2. **Run Rancher against the new cluster.** Rancher reads `~/.kube/config`, which now points at
   `k3d-local-caprke2`. Start it however you normally do — the GoLand run target, or:
   ```shell
   ./dev-scripts/quick
   ```
   Rancher will install Turtles into the cluster on first startup.

3. **Install the CAPRKE2 + CAPD providers** once Rancher is up:
   ```shell
   make install-caprke2-providers
   ```
   Waits for Turtles' `capiproviders.turtles-capi.cattle.io` CRD to appear (~1–2 min after
   Rancher boots), then applies `tests/v2prov/defaults/caprke2-providers.yaml` and waits for
   the `rke2-bootstrap`, `rke2-control-plane`, and `capd` controller-managers to be Ready.

4. **Run the test:**
   ```shell
   V2PROV_TEST_CAPRKE2=true go test -v -failfast -timeout 60m \
     -run '^Test_Operation_SetE_CAPRKE2DockerOperations$' \
     ./tests/v2prov/tests/imported/...
   ```

5. **Tear down** when finished:
   ```shell
   make dev-env-cleanup
   ```
   Deletes the k3d cluster; leaves the `kind` docker network if any other containers are still
   attached to it.

### Overrides

- `CAPRKE2_DEV_CLUSTER` — k3d cluster name (default: `local-caprke2`). Applies to both
  `make dev-env` and `make dev-env-cleanup`.
- `K3S_VERSION` — k3s image tag (default: `v1.33.5-k3s1`).
- `V2PROV_TEST_CAPRKE2_MANIFEST` — absolute path to a local providers manifest; applied
  verbatim by `make install-caprke2-providers` in place of the in-repo default.
- `V2PROV_TEST_CAPRKE2_TURTLES_REF` — `rancher/turtles` git ref (tag/branch/SHA); the
  `charts/rancher-turtles-providers` chart is `helm template`-ed at that ref instead. Useful
  for testing against an upstream Turtles version other than the one pinned in this repo.
