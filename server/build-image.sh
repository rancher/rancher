#!/bin/bash
set -e -x

cd $(dirname $0)

if [ ! -e target/.done ]; then
    mkdir -p target
    docker run -it -v $(pwd)/target:/output rancher/s6-builder:v0.1.0 /opt/build.sh
    touch target/.done
fi

TAG=${TAG:-$(awk '/CATTLE_RANCHER_SERVER_IMAGE/{print $3}' Dockerfile)}
IMAGE=rancher/server:${TAG}

docker build -t ${IMAGE} .

echo Done building ${IMAGE}
