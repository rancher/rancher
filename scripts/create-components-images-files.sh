#!/usr/bin/env bash
 set -e

cd $(dirname $0)

source version
source export-config
source package-env

cd ../package

if [ ${ARCH} == arm64 ]; then
    ETCD_UNSUPPORTED_ARCH=arm64
fi

mkdir -p ../dist

cd ../bin

if [ ! -d $CHART_REPO_DIR ]; then
    git clone --branch $CHART_DEFAULT_BRANCH https://github.com/rancher/charts $CHART_REPO_DIR
fi

if [ ! -d $SMALL_FORK_REPO_DIR ]; then
    mkdir -p $SMALL_FORK_REPO_DIR
    git clone --branch main https://github.com/rancher/charts-small-fork $SMALL_FORK_REPO_DIR
fi

if [ ${ARCH} == amd64 ]; then
    # Move this out of ARCH check for local dev on non-amd64 hardware.
    TAG=$TAG REPO=${REPO} go run ../pkg/image/export/main.go $SYSTEM_CHART_REPO_DIR $CHART_REPO_DIR $IMAGE $AGENT_IMAGE $SYSTEM_AGENT_UPGRADE_IMAGE $WINS_AGENT_UPGRADE_IMAGE ${SYSTEM_AGENT_INSTALLER_RKE2_IMAGES[@]} ${SYSTEM_AGENT_INSTALLER_K3S_IMAGES[@]}
fi

RKE2_LINUX_RUNTIME_IMAGES=(
  "rancher/rke2-runtime:v1.30.10-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.30.11-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.30.12-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.30.13-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.30.14-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.30.14-rke2r2-linux-amd64"
  "rancher/rke2-runtime:v1.30.9-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.10-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.11-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.12-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.13-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.14-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.5-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.6-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.7-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.8-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.31.9-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.1-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.10-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.11-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.2-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.3-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.4-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.5-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.6-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.7-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.8-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.32.9-rke2r1-linux-amd64"
  "rancher/rke2-runtime:v1.30.10-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.30.11-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.30.12-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.30.13-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.30.14-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.30.14-rke2r2-linux-arm64"
  "rancher/rke2-runtime:v1.30.9-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.10-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.11-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.12-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.13-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.14-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.5-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.6-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.7-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.8-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.31.9-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.1-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.10-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.11-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.2-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.3-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.4-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.5-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.6-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.7-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.8-rke2r1-linux-arm64"
  "rancher/rke2-runtime:v1.32.9-rke2r1-linux-arm64"
)

RKE2_WINDOWS_RUNTIME_IMAGES=(
  "rancher/rke2-runtime:v1.30.10-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.30.11-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.30.12-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.30.13-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.30.14-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.30.14-rke2r2-windows-amd64"
  "rancher/rke2-runtime:v1.30.9-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.10-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.11-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.12-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.13-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.14-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.5-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.6-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.7-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.8-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.31.9-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.1-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.10-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.11-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.2-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.3-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.4-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.5-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.6-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.7-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.8-rke2r1-windows-amd64"
  "rancher/rke2-runtime:v1.32.9-rke2r1-windows-amd64"
)

for image in "${RKE2_LINUX_RUNTIME_IMAGES[@]}"; do
  echo "$image" >>"rancher-images.txt"
  echo "$image rancher,rke2All" >>"rancher-images-sources.txt"
done

for image in "${RKE2_WINDOWS_RUNTIME_IMAGES[@]}"; do
  echo "$image" >>"rancher-windows-images.txt"
  echo "$image rancher,rke2All" >>"rancher-windows-images-sources.txt"
done

# Create components file used for pre-release notes
../scripts/create-components-file.sh
