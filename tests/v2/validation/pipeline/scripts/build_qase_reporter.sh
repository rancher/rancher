#!/bin/bash
set -e
cd $(dirname $0)/../../../../../
if [[ -z "${QASE_TEST_RUN_ID}" ]]; then
  echo "no test run ID is provided"
else
  echo "building qase reporter bin"
  env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tests/v2/validation/reporter ./tests/v2/validation/pipeline/qase/reporter
fi