#!/bin/bash
set -e
cd $(dirname $0)/../../../../


echo "building cluster setup bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/codecoverage/bin/setuprancher ./tests/v2/codecoverage/setuprancher

echo "building rancher HA corral bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/codecoverage/bin/ranchercorral ./tests/v2/codecoverage/rancherha

echo "building rancher cleanup bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/codecoverage/bin/ranchercleanup ./tests/v2/codecoverage/ranchercleanup
