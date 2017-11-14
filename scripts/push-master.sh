#!/bin/bash
set -e
cd $(dirname $0)/..
./scripts/build
TAG=master ./scripts/package
docker push rancher/cattle:master
