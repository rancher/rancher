#!/bin/bash

cd $(dirname $0)

if [ -z "$TAG" ]; then
    TAG=$(grep RANCHER_AGENT_IMAGE Dockerfile | cut -f2 -d:)
fi

IMAGE=rancher/agent:${TAG}

echo Building $IMAGE
docker build -t ${IMAGE} .
