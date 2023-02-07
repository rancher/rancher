#!/bin/bash
set -e
cd $(dirname $0)/../../../../


echo "building rancher HA corral bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/registries/bin/ranchercorral ./tests/v2/registries/rancherha

echo "building cluster setup bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/registries/bin/setuprancher ./tests/v2/registries/setuprancher

echo "building registry auth enabled bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/registries/bin/registryauthenabled ./tests/v2/registries/registryauthenabled

echo "building registry auth disabled bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/registries/bin/registryauthdisabled ./tests/v2/registries/registryauthdisabled

echo "building rancher cleanup bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/registries/bin/ranchercleanup ./tests/v2/registries/ranchercleanup
