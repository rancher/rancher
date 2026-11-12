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

# The image lists must match the charts baked into the Rancher Assets image in use, so resolve the charts commit
# from the assets repo's lock.yaml at the tag the image was built from, instead of using the branch head.
ASSETS_IMAGE=${ASSETS_IMAGE:-$(build_yaml_get defaultAssetsImage)}
ASSETS_TAG=${ASSETS_IMAGE##*:}   # e.g. v2.16-20261027T2136Z-dev
ASSETS_MINOR=${ASSETS_TAG#v}
ASSETS_MINOR=${ASSETS_MINOR%%-*} # e.g. 2.16
if [[ $ASSETS_TAG == *-dev ]]; then ASSETS_ENV=dev; else ASSETS_ENV=prod; fi

ASSETS_REPO_DIR=build/rancher-assets
if [ ! -d $ASSETS_REPO_DIR ]; then
    git clone --depth 1 --branch $ASSETS_TAG https://github.com/rancher/rancher-assets $ASSETS_REPO_DIR
fi

# Reads chart-versions.<minor>.upstream-refs.<env>.charts.commit from lock.yaml.
CHARTS_COMMIT=$(awk -v minor="$ASSETS_MINOR" -v env="$ASSETS_ENV" '
    { match($0, /^ */); indent = RLENGTH; key = $1; gsub(/[":]/, "", key) }
    indent == 0 { in_versions = (key == "chart-versions"); in_minor = in_env = in_charts = 0; next }
    !in_versions { next }
    indent == 4 { in_minor = (key == minor); in_env = in_charts = 0; next }
    !in_minor { next }
    indent == 12 { in_env = (key == env); in_charts = 0; next }
    !in_env { next }
    indent == 16 { in_charts = (key == "charts"); next }
    in_charts && indent == 20 && key == "commit" { print $2; exit }
' $ASSETS_REPO_DIR/lock.yaml)
if [ -z "$CHARTS_COMMIT" ]; then
    echo "Could not find charts commit for ${ASSETS_MINOR}/${ASSETS_ENV} in $ASSETS_REPO_DIR/lock.yaml (assets tag ${ASSETS_TAG})" >&2
    exit 1
fi

if [ ! -d $CHART_REPO_DIR ]; then
    git clone --branch $CHART_DEFAULT_BRANCH https://github.com/rancher/charts $CHART_REPO_DIR
fi
git -C $CHART_REPO_DIR fetch origin $CHARTS_COMMIT
git -C $CHART_REPO_DIR checkout --detach $CHARTS_COMMIT

CHARTS_DIRS="${CHARTS_DIRS:+${CHARTS_DIRS},}${CHART_REPO_DIR}"

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
