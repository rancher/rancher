#!/bin/bash
set -e

cd $(dirname $0)/../../../../

source $(dirname $0)/scripts/version
source $(dirname $0)/scripts/export-config

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

CGO_ENABLED=0 GOCOVERDIR=ranchercoverage GOOS=linux GOARCH=amd64 go build -tags k8s  -cover -gcflags="all=${GCFLAGS}" -ldflags "-X main.VERSION=$VERSION $LINKFLAGS" -o tests/v2/codecoverage/bin/agent ././tests/v2/codecoverage/agent
