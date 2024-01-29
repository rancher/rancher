#!/bin/bash
set -ex

cd $(dirname $0)/../../../../

source $(dirname $0)/scripts/version
source $(dirname $0)/scripts/export-config

CATTLE_KDM_BRANCH=dev-v2.8

mkdir -p bin

if [ -n "${DEBUG}" ]; then
  GCFLAGS="-N -l"
fi

if [ "$(uname)" != "Darwin" ]; then
  LINKFLAGS="-extldflags -static"
  if [ -z "${DEBUG}" ]; then
    LINKFLAGS="${LINKFLAGS} -s"
  fi
fi

RKE_VERSION="$(grep -m1 'github.com/rancher/rke' go.mod | awk '{print $2}')"

# Inject Setting values
DEFAULT_VALUES="{\"rke-version\":\"${RKE_VERSION}\"}"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags k8s \
  -cover \
  -gcflags="all=${GCFLAGS}" \
  -ldflags \
  "-X github.com/rancher/rancher/pkg/version.Version=$VERSION
   -X github.com/rancher/rancher/pkg/version.GitCommit=$COMMIT
   -X github.com/rancher/rancher/pkg/settings.InjectDefaults=$DEFAULT_VALUES $LINKFLAGS" \
  -o tests/v2/codecoverage/bin \
  ./tests/v2/codecoverage/rancher
 
curl -sLf https://raw.githubusercontent.com/chiukapoor/kontainer-driver-metadata/rancher-v1.28/data/data.json > tests/v2/codecoverage/bin/data.json