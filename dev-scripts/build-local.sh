#!/usr/bin/env bash

# Script to build a Rancher local image with magic for cross-building for different platforms.
# Can be improved to build a multiarch image instead.

set -eo pipefail
set -x

SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"
source "$SCRIPT_DIR/../scripts/export-config"

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
    curl -sLf https://releases.rancher.com/kontainer-driver-metadata/release-v2.7/data.json > "${DATA_JSON_FILE}"
fi

K3S_AIRGAP_IMAGES_TARBALL="${SCRIPT_DIR}/../bin/k3s-airgap-images.tar"
if [ ! -f "${K3S_AIRGAP_IMAGES_TARBALL}" ]; then
    touch "${K3S_AIRGAP_IMAGES_TARBALL}"
fi

PACKAGE_FOLDER="${SCRIPT_DIR}/../package/"
cp "${RANCHER_BINARY}" "${PACKAGE_FOLDER}"
cp "${DATA_JSON_FILE}" "${PACKAGE_FOLDER}"
cp "${K3S_AIRGAP_IMAGES_TARBALL}" "${PACKAGE_FOLDER}"

DOCKERFILE="${SCRIPT_DIR}/../package/Dockerfile"
# Always use buildx to make sure the image & the binary architectures match
docker buildx build -t "${TARGET_REPO}" -f "${DOCKERFILE}" \
  --build-arg CATTLE_RANCHER_WEBHOOK_VERSION="${CATTLE_RANCHER_WEBHOOK_VERSION}" \
  --build-arg CATTLE_CSP_ADAPTER_MIN_VERSION="${CATTLE_CSP_ADAPTER_MIN_VERSION}" \
  --build-arg CATTLE_FLEET_VERSION="${CATTLE_FLEET_VERSION}" \
  --build-arg ARCH="${TARGET_ARCH}" \
  "${PACKAGE_FOLDER}" --platform="${TARGET_OS}/${TARGET_ARCH}"
