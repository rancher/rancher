#!/bin/bash
set -e
cd $(dirname $0)/../../../../../

  echo "building qase auto testrun bin"
  env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/testrun ./tests/v2/validation/pipeline/qase/testrun