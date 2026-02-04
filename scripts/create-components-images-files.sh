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

CHARTS_DIRS="${CHARTS_DIRS:+${CHARTS_DIRS},}${CHART_REPO_DIR}"

if [ ! -d $SMALL_FORK_REPO_DIR ]; then
    mkdir -p $SMALL_FORK_REPO_DIR
    git clone --branch main https://github.com/rancher/charts-small-fork $SMALL_FORK_REPO_DIR
fi

if [ ${ARCH} == amd64 ]; then
    # Move this out of ARCH check for local dev on non-amd64 hardware.
    TAG=$TAG REPO=${REPO} go run ../pkg/image/export/main.go $CHARTS_DIRS $IMAGE $AGENT_IMAGE $SYSTEM_AGENT_UPGRADE_IMAGE $WINS_AGENT_UPGRADE_IMAGE $CLUSTER_API_CONTROLLER_IMAGE ${SYSTEM_AGENT_INSTALLER_RKE2_IMAGES[@]} ${SYSTEM_AGENT_INSTALLER_K3S_IMAGES[@]}
fi

# Create components file used for pre-release notes
../scripts/create-components-file.sh
