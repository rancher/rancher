#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"

env | egrep '^(ARM_|CATTLE_|ADMIN|USER|DO|RANCHER_|K3S_|RKE2_|AWS_|DEBUG|LOGLEVEL|DEFAULT_|OS_|DOCKER_|CLOUD_|KUBE|BUILD_NUMBER|RKE_|AZURE).*\=.+' | sort > .env

if [ "false" != "${DEBUG}" ]; then
    cat .env
fi
