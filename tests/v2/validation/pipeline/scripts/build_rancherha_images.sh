#!/bin/bash
set -e
cd $(dirname $0)/../../../../../

echo "building rancher HA corral bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/registries/bin/rancherha ./tests/v2/validation/pipeline/rancherha

echo "building rancher cleanup bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/registries/bin/ranchercleanup ./tests/v2/validation/pipeline/ranchercleanup
