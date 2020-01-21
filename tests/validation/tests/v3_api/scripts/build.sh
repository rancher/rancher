#!/bin/bash

set -x
set -e

DEBUG="${DEBUG:-false}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.9.0}"

if [ "false" != "${DEBUG}" ]; then
    echo "Environment:"
    env | sort
fi

cd "$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../../" && pwd )"

count=0
while [[ 3 -gt $count ]]; do
    if [ -z "${RANCHER_RKE_VERSION}" ]; then
        RKE_STRING=""
    else
        RKE_STRING="--build-arg RKE_VERSION=${RANCHER_RKE_VERSION}"
    fi
    
    docker build -q -f Dockerfile.v3api $RKE_STRING -t rancher-validation-${JOB_NAME}${BUILD_NUMBER} .

    if [[ $? -eq 0 ]]; then break; fi
    count=$(($count + 1))
    echo "Repeating failed Docker build ${count} of 3..."
done
