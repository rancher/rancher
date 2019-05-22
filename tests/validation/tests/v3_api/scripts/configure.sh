#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"

env | egrep '^(CATTLE_|ADMIN|USER|DO|RANCHER_|AWS_|DEBUG|DEFAULT_|OS_|DOCKER_|CLOUD_|KUBE|BUILD_NUMBER|AZURE).*\=.+' | sort > .env

if [ "false" != "${DEBUG}" ]; then
    cat .env
fi
