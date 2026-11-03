#!/usr/bin/env bash
# Smoke test the Rancher server image the way it is installed in production:
# Helm chart into a fresh k3d cluster, CATTLE_DEV_MODE unset. Then check that
# Rancher and the system components it installs come up.
#
# Usage:
#   IMAGE=rancher/rancher:<tag> [RANCHER_CHART=path/to/rancher.tgz] .github/scripts/smoke-test.sh
#
# If RANCHER_CHART is unset, the chart is built from ./chart.
set -euo pipefail

cd "$(dirname "$0")/../.."

IMAGE=${IMAGE:?IMAGE must be set to the local server image, e.g. rancher/rancher:v2.16-abc-head}
RANCHER_CHART=${RANCHER_CHART:-}
CLUSTER_NAME=${CLUSTER_NAME:-rancher-smoke}
REGISTRY_NAME=${REGISTRY_NAME:-rancher-smoke-registry}
REGISTRY_PORT=${REGISTRY_PORT:-5055}
# [host:]port the k3d API server is published on. Set a host (e.g. the docker
# bridge gateway) when the docker daemon's localhost isn't reachable from here.
K3D_API_PORT=${K3D_API_PORT:-6550}
K3S_VERSION=${K3S_VERSION:-$(grep -m1 'ENV CATTLE_K3S_VERSION=' package/Dockerfile | cut -d '=' -f2)}
CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-v1.15.3}
TIMEOUT=${TIMEOUT:-600}
KEEP_CLUSTER=${KEEP_CLUSTER:-false}

WORKDIR=build/smoke-test
mkdir -p "$WORKDIR"
export KUBECONFIG=$PWD/$WORKDIR/kubeconfig

log() { echo "== $* =="; }

dump_logs() {
  log "Pods"
  kubectl get pods -A || true
  log "Helm releases"
  helm list -A || true
  log "Rancher logs"
  kubectl logs -n cattle-system -l app=rancher --tail=-1 || true
}

cleanup() {
  rc=$?
  if [ "$rc" -ne 0 ]; then
    dump_logs
  fi
  if [ "$KEEP_CLUSTER" != true ]; then
    k3d cluster delete "$CLUSTER_NAME" || true
  else
    echo "Keeping cluster $CLUSTER_NAME (KUBECONFIG=$KUBECONFIG)"
  fi
  exit "$rc"
}

# check <description> <command>: retries <command> until it succeeds or TIMEOUT.
check() {
  echo "-- check: $1"
  ./scripts/retry --timeout "$TIMEOUT" --sleep 5 --message-interval 12 \
    --message "$1: still waiting after {{elapsed}}s" "$2"
  echo "   ok: $1"
}

log "Registry k3d-$REGISTRY_NAME"
if ! k3d registry list "k3d-$REGISTRY_NAME" >/dev/null 2>&1; then
  k3d registry create "$REGISTRY_NAME" --port "$REGISTRY_PORT"
fi

log "Push $IMAGE"
IMAGE_PATH=${IMAGE#*/}
IMAGE_REPO=${IMAGE_PATH%:*}
IMAGE_TAG=${IMAGE##*:}
# Pushed through the published port, pulled in-cluster via the registry name.
docker tag "$IMAGE" "127.0.0.1:$REGISTRY_PORT/rancher/$IMAGE_REPO:$IMAGE_TAG"
./scripts/retry --timeout 60 --sleep 2 \
  docker push "127.0.0.1:$REGISTRY_PORT/rancher/$IMAGE_REPO:$IMAGE_TAG"
CLUSTER_IMAGE_REPO="k3d-$REGISTRY_NAME:$REGISTRY_PORT/rancher/$IMAGE_REPO"

log "Cluster $CLUSTER_NAME (k3s $K3S_VERSION)"
k3d cluster create "$CLUSTER_NAME" \
  --image "rancher/k3s:${K3S_VERSION//+/-}" \
  --api-port "$K3D_API_PORT" \
  --registry-use "k3d-$REGISTRY_NAME:$REGISTRY_PORT" \
  --k3s-arg "--disable=metrics-server@server:*" \
  --kubeconfig-update-default=false \
  --wait
trap cleanup EXIT
k3d kubeconfig get "$CLUSTER_NAME" > "$KUBECONFIG"
kubectl get nodes

log "cert-manager $CERT_MANAGER_VERSION"
kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
kubectl rollout status -n cert-manager deploy --timeout=300s

if [ -z "$RANCHER_CHART" ]; then
  log "Build chart"
  ./scripts/chart/build chart
  RANCHER_CHART=build/chart/rancher
fi

log "Install Rancher ($RANCHER_CHART)"
helm upgrade --install rancher "$RANCHER_CHART" \
  --namespace cattle-system --create-namespace \
  --set hostname=rancher.smoke.test \
  --set bootstrapPassword=adminadminadmin \
  --set replicas=1 \
  --set image.repository="$CLUSTER_IMAGE_REPO" \
  --set image.tag="$IMAGE_TAG" \
  --wait --timeout 10m

log "Checks"
check "rancher deployment rolled out" \
  "kubectl rollout status -n cattle-system deploy/rancher --timeout=10s"
check "rancher /ping" \
  "kubectl get --raw /api/v1/namespaces/cattle-system/services/http:rancher:80/proxy/ping | grep -qx pong"
# Only created outside of CATTLE_DEV_MODE, by package/jailer.sh.
check "driver jail created (/opt/jail/driver-jail/done)" \
  "kubectl exec -n cattle-system deploy/rancher -c rancher -- test -f /opt/jail/driver-jail/done"
# No api-extension (remotedialer-proxy) check: the chart sets
# IMPERATIVE_API_DIRECT=true, so it is not deployed for Helm installs.
check "rancher-webhook deployment" \
  "kubectl rollout status -n cattle-system deploy/rancher-webhook --timeout=10s"
check "fleet-controller deployment" \
  "kubectl rollout status -n cattle-fleet-system deploy/fleet-controller --timeout=10s"
check "gitjob deployment" \
  "kubectl rollout status -n cattle-fleet-system deploy/gitjob --timeout=10s"
check "fleet-agent (local cluster) deployment" \
  "kubectl rollout status -n cattle-fleet-local-system deploy/fleet-agent --timeout=10s"
check "rancher-turtles deployment" \
  "kubectl rollout status -n cattle-turtles-system deploy/rancher-turtles-controller-manager --timeout=10s"
check "capi-controller-manager deployment" \
  "kubectl rollout status -n cattle-capi-system deploy/capi-controller-manager --timeout=10s"
# No system-upgrade-controller check: it is only installed when the local
# cluster's driver is detected as k3s/rke2, which isn't deterministic here.
check "local cluster Ready" \
  "kubectl get clusters.management.cattle.io local -o jsonpath='{.status.conditions[?(@.type==\"Ready\")].status}' | grep -qx True"

log "Smoke test passed"
