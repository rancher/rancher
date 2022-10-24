#!/bin/bash
set -e
cd $(dirname $0)/../../../../../

echo "building release downstream cleanup bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/pipeline/bin/downstreamcleanup ./tests/v2/validation/pipeline/downstreamcleanup

echo "running downstream cleanup"
tests/v2/validation/pipeline/bin/downstreamcleanup
