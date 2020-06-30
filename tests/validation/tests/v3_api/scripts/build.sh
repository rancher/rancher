#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"
RKE_VERSION="${RKE_VERSION:-v1.0.2}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.16.6}"
CLI_VERSION="${CLI_VERSION:-v2.4.5}"
SONOBUOY_VERSION="${SONOBUOY_VERSION:-0.18.2}"

if [ "false" != "${DEBUG}" ]; then
    echo "Environment:"
    env | sort
fi

cd "$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../../" && pwd )"

count=0
while [[ 3 -gt $count ]]; do    
    docker build -q -f Dockerfile.v3api --build-arg CLI_VERSION=$CLI_VERSION --build-arg RKE_VERSION=$RKE_VERSION --build-arg KUBECTL_VERSION=$KUBECTL_VERSION --build-arg SONOBUOY_VERSION=$SONOBUOY_VERSION -t rancher-validation-${JOB_NAME}${BUILD_NUMBER} .

    if [[ $? -eq 0 ]]; then break; fi
    count=$(($count + 1))
    echo "Repeating failed Docker build ${count} of 3..."
done
