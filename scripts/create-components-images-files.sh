#!/usr/bin/env bash
 set -e

cd $(dirname $0)

source version
source package-env

cd ../package

if [ ${ARCH} == arm64 ]; then
    ETCD_UNSUPPORTED_ARCH=arm64
fi

mkdir -p ../dist

cd ../bin

# Image info comes from the helm.sh/images annotations in the branch's index.yaml.
# Valid branches are "staging" and "production".
CHARTS_BRANCH=${CHARTS_BRANCH:-staging}
CHARTS_INDEX_DIR=build/charts-${CHARTS_BRANCH}
if [ ! -d $CHARTS_INDEX_DIR ]; then
    git clone --depth 1 --branch $CHARTS_BRANCH https://github.com/rancher/charts $CHARTS_INDEX_DIR
fi

CHARTS_DIRS="${CHARTS_DIRS:+${CHARTS_DIRS},}${CHARTS_INDEX_DIR}"

if [ ! -d $SMALL_FORK_REPO_DIR ]; then
    mkdir -p $SMALL_FORK_REPO_DIR
    git clone --branch main https://github.com/rancher/charts-small-fork $SMALL_FORK_REPO_DIR
fi

if [ ${ARCH} == amd64 ]; then
    # Move this out of ARCH check for local dev on non-amd64 hardware.
    TAG=$TAG REPO=${REPO} go run ../pkg/image/export/main.go $CHARTS_DIRS $IMAGE $AGENT_IMAGE $SYSTEM_AGENT_UPGRADE_IMAGE $WINS_AGENT_UPGRADE_IMAGE $CLUSTER_API_CONTROLLER_IMAGE
fi

# Create components file used for pre-release notes
../scripts/create-components-file.sh
