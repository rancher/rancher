#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"

env | egrep '^(ARM_|CATTLE_|ADMIN|USER|DO|RANCHER_|AWS_|DEBUG|LOGLEVEL|DEFAULT_|OS_|DOCKER_|CLOUD_|KUBE|BUILD_NUMBER|AZURE|TEST_|QASE_|SLACK_).*\=.+' | sort > .env

if [ "false" != "${DEBUG}" ]; then
    cat .env
fi