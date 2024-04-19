#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"
CLI_VERSION="${CLI_VERSION}"
EXTERNAL_ENCODED_VPN="${EXTERNAL_ENCODED_VPN}"
VPN_ENCODED_LOGIN="${VPN_ENCODED_LOGIN}"

TRIM_JOB_NAME=$(basename "$JOB_NAME")

if [ "false" != "${DEBUG}" ]; then
    echo "Environment:"
    env | sort
fi

count=0
while [[ 3 -gt $count ]]; do
    docker build . -f tests/v2/validation/Dockerfile.validation --build-arg CLI_VERSION="$CLI_VERSION" --build-arg EXTERNAL_ENCODED_VPN="$EXTERNAL_ENCODED_VPN" --build-arg VPN_ENCODED_LOGIN="$VPN_ENCODED_LOGIN" -t rancher-validation-"${TRIM_JOB_NAME}""${BUILD_NUMBER}"   

    if [[ $? -eq 0 ]]; then break; fi
    count=$(($count + 1))
    echo "Repeating failed Docker build ${count} of 3..."
done