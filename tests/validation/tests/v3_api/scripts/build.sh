#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"
RKE_VERSION="${RKE_VERSION:-v1.0.2}"
RANCHER_HELM_VERSION="${RANCHER_HELM_VERSION:-v3.8.0}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.27.10}"
CLI_VERSION="${CLI_VERSION:-v2.4.5}"
SONOBUOY_VERSION="${SONOBUOY_VERSION:-0.18.2}"
TERRAFORM_VERSION="${TERRAFORM_VERSION:-0.12.10}"
EXTERNAL_ENCODED_VPN="${EXTERNAL_ENCODED_VPN:-1234}"
VPN_ENCODED_LOGIN="${VPN_ENCODED_LOGIN:-5678}"

TRIM_JOB_NAME=$(basename "$JOB_NAME")

if [ "false" != "${DEBUG}" ]; then
    echo "Environment:"
    env | sort
fi

cd "$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../../" && pwd )"

count=0
while [[ 3 -gt $count ]]; do    
    docker build -q -f Dockerfile.v3api --build-arg CLI_VERSION="$CLI_VERSION" --build-arg RKE_VERSION="$RKE_VERSION" --build-arg RANCHER_HELM_VERSION="$RANCHER_HELM_VERSION" --build-arg KUBECTL_VERSION="$KUBECTL_VERSION" --build-arg SONOBUOY_VERSION="$SONOBUOY_VERSION" --build-arg TERRAFORM_VERSION="$TERRAFORM_VERSION" --build-arg EXTERNAL_ENCODED_VPN="$EXTERNAL_ENCODED_VPN" --build-arg VPN_ENCODED_LOGIN="$VPN_ENCODED_LOGIN" -t rancher-validation-"${TRIM_JOB_NAME}""${BUILD_NUMBER}" .

    if [[ $? -eq 0 ]]; then break; fi
    count=$(($count + 1))
    echo "Repeating failed Docker build ${count} of 3..."
done
