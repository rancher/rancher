#!/bin/bash

cd $(dirname $0)

if [ -z "$IMAGE" ]; then
    IMAGE=$(grep RANCHER_AGENT_IMAGE Dockerfile | awk '{print $3}')
fi

echo Building $IMAGE
docker build -t ${IMAGE} .
