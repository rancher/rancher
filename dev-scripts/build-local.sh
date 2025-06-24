#!/usr/bin/env bash

# Script to build a Rancher local image with magic for cross-building for different platforms.
# Can be improved to build a multiarch image instead.

set -eo pipefail
set -x

SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"

TARGET_OS="${TARGET_OS:-linux}"
GO_BINARY="${GO_BINARY:-$(which go)}"

if [ -z "${TARGET_REPO}" ]; then
  echo "please specify the variable TARGET_REPO"
  exit 1
fi

# If not defined assume it's being built for the current arch.
# Currently just supporting arm64 and amd64 but could add more in the future.
if [ -z "${TARGET_ARCH}" ]; then
  case "$(uname -m)" in
    arm64|aarch64)
      TARGET_ARCH="arm64"
      ;;
    *)
      TARGET_ARCH="amd64"
      ;;
  esac
fi

set -u

RKE_VERSION="$(grep -m1 'github.com/rancher/rke' "${SCRIPT_DIR}/../go.mod" | awk '{print $2}')"
DEFAULT_VALUES="{\"rke-version\":\"${RKE_VERSION}\"}"

RANCHER_BINARY="${SCRIPT_DIR}/../bin/rancher"
GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" CGO_ENABLED=0 "${GO_BINARY}" build -tags k8s \
    -ldflags "-X github.com/rancher/rancher/pkg/version.Version=dev -X github.com/rancher/rancher/pkg/version.GitCommit=dev -X github.com/rancher/rancher/pkg/settings.InjectDefaults=$DEFAULT_VALUES -extldflags -static -s" \
    -o "${RANCHER_BINARY}"

DATA_JSON_FILE="${SCRIPT_DIR}/../bin/data.json"
if [ ! -f "${DATA_JSON_FILE}" ]; then
    curl -sLf https://releases.rancher.com/kontainer-driver-metadata/release-v2.9/data.json > "${DATA_JSON_FILE}"
fi

K3S_AIRGAP_IMAGES_TARBALL="${SCRIPT_DIR}/../bin/k3s-airgap-images.tar"
if [ ! -f "${K3S_AIRGAP_IMAGES_TARBALL}" ]; then
    touch "${K3S_AIRGAP_IMAGES_TARBALL}"
fi

PACKAGE_FOLDER="${SCRIPT_DIR}/../package/"
case $PWD in
    */rancher/rancher$) PACKAGE_FOLDER=package ;;
esac

cp "${RANCHER_BINARY}" "${PACKAGE_FOLDER}/"
cp "${DATA_JSON_FILE}" "${PACKAGE_FOLDER}/"
cp "${K3S_AIRGAP_IMAGES_TARBALL}" "${PACKAGE_FOLDER}/"

echo "QQQ: PACKAGE_FOLDER: $PACKAGE_FOLDER"
DOCKERFILE="${PACKAGE_FOLDER}/Dockerfile-main-dev"

CATTLE_CSP_ADAPTER_MIN_VERSION=$(yq '.cspAdapterMinVersion' < build.yaml)
CATTLE_FLEET_VERSION=$(yq '.fleetVersion' < build.yaml)
CATTLE_K3S_VERSION=$(grep -m1 'ENV CATTLE_K3S_VERSION' package/Dockerfile | awk '{print $3}')
CATTLE_KDM_BRANCH=$(grep -m1 'ARG CATTLE_KDM_BRANCH=' package/Dockerfile | cut -d '=' -f2)
CATTLE_RANCHER_PROVISIONING_CAPI_VERSION=$(yq '.provisioningCAPIVersion' < build.yaml)
CATTLE_RANCHER_WEBHOOK_VERSION=$(yq '.webhookVersion' < build.yaml)
CATTLE_REMOTEDIALER_PROXY_VERSION=$(yq '.remoteDialerProxyVersion' < build.yaml)

RKE_VERSION=$(grep -m1 'github.com/rancher/rke' go.mod | awk '{print $2}')
if [[ -z "$RKE_VERSION" ]]; then
   RKE_VERSION=$(grep -m1 'github.com/rancher/rke' go.mod | awk '{print $4}')
fi

# Always use buildx to make sure the image & the binary architectures match
ATAG="$(echo $TAG | sed s/arm64/amd64/g)"

echo "TAG is now: ${TAG}"
echo tag-thing is now: "${TARGET_REPO}"
# exit 2

docker buildx build \
    --build-arg IMAGE_REPO=$REPO \
    --build-arg VERSION=$TAG \
    --build-arg AVERSION="$ATAG" \
    --build-arg RKE_VERSION="${RKE_VERSION}" \
    --build-arg CATTLE_CSP_ADAPTER_MIN_VERSION="${CATTLE_CSP_ADAPTER_MIN_VERSION}" \
    --build-arg CATTLE_FLEET_VERSION="${CATTLE_FLEET_VERSION}" \
    --build-arg CATTLE_K3S_VERSION="${CATTLE_K3S_VERSION}" \
    --build-arg CATTLE_RANCHER_PROVISIONING_CAPI_VERSION="${CATTLE_RANCHER_PROVISIONING_CAPI_VERSION}" \
    --build-arg CATTLE_RANCHER_WEBHOOK_VERSION="${CATTLE_RANCHER_WEBHOOK_VERSION}" \
    --build-arg CATTLE_REMOTEDIALER_PROXY_VERSION="${CATTLE_REMOTEDIALER_PROXY_VERSION}" \
    --tag "${TARGET_REPO}" \
    --platform="${TARGET_OS}/${TARGET_ARCH}" \
    --file "${DOCKERFILE}" "${PACKAGE_FOLDER}"

# ???? Not sure we need the second 'docker build' command....
# Work at the repo top level
#docker build -t "${TARGET_REPO}" -f "${DOCKERFILE}" . --platform="${TARGET_OS}/${TARGET_ARCH}"
