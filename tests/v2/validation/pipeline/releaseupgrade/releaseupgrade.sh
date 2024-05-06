#!/bin/bash
set -e
cd $(dirname $0)/../../../../../

echo "building release upgrade bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/pipeline/bin/releaseupgrade ./tests/v2/validation/pipeline/releaseupgrade

echo "running release upgrade"
tests/v2/validation/pipeline/bin/releaseupgrade
