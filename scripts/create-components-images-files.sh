#!/usr/bin/env bash
 set -e

cd $(dirname $0)

source package-env

cd ../package

# Query KDM data for RKE2 released versions where server args are defined.
RKE2_RELEASE_VERSIONS=$(jq -r '[.rke2.releases[] | select(.serverArgs) | .version] | join(" ")' bin/data.json)
# Convert versions with build metadata into valid image tags (replace + for -) and construct an array of tags.
RKE2_RELEASE_TAGS=($(echo $RKE2_RELEASE_VERSIONS | tr + -))
# Prefix image repo and name to tags.
SYSTEM_AGENT_INSTALLER_RKE2_IMAGES=("${RKE2_RELEASE_TAGS[@]/#/${REPO}/system-agent-installer-rke2:}")

# Query KDM data for K3S released versions where server args are defined.
K3S_RELEASE_VERSIONS=$(jq -r '[.k3s.releases[] | select(.serverArgs) | .version] | join(" ")' bin/data.json)
# Convert versions with build metadata into valid image tags (replace + for -) and construct an array of tags.
K3S_RELEASE_TAGS=($(echo $K3S_RELEASE_VERSIONS | tr + -))
# Prefix image repo and name to tags.
SYSTEM_AGENT_INSTALLER_K3S_IMAGES=("${K3S_RELEASE_TAGS[@]/#/${REPO}/system-agent-installer-k3s:}")

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

# Create components file used for pre-release notes
../scripts/create-components-file.sh
