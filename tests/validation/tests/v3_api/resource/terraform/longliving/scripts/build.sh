#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"
TERRAFORM_VERSION="${TERRAFORM_VERSION:-1.0.11}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.24.0}"

TRIM_JOB_NAME=$(basename "$JOB_NAME")

if [ "false" != "${DEBUG}" ]; then
    echo "Environment:"
    env | sort
fi

cd "$( cd "$( dirname "${BASH_SOURCE[0]}" )/../" && pwd )"

count=0
while [[ 3 -gt $count ]]; do    
    docker build -q -f Dockerfile.longliving --build-arg TERRAFORM_VERSION="$TERRAFORM_VERSION" --build-arg KUBECTL_VERSION="$KUBECTL_VERSION" -t rancher-longliving-"${TRIM_JOB_NAME}""${BUILD_NUMBER}" .

    if [[ $? -eq 0 ]]; then break; fi
    count=$(($count + 1))
    echo "Repeating failed Docker build ${count} of 3..."
done
