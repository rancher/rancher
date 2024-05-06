#!/bin/bash
set -e
cd $(dirname $0)/../../../../../

configPath=$CATTLE_TEST_CONFIG

echo "building ha post install bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/pipeline/bin/hapostinstall ./tests/v2/validation/pipeline/hapostinstall

echo "running ha post install"
tests/v2/validation/pipeline/bin/hapostinstall

export CATTLE_TEST_CONFIG=$configPath
