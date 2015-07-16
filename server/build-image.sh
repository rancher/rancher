#!/bin/bash
set -e -x

cd $(dirname $0)

if [ ! -e target/.done ]; then
    mkdir -p target
    docker run -it -v $(pwd)/target:/output rancher/s6-builder:v0.1.0 /opt/build.sh
    touch target/.done
fi

TAG=${TAG:-$(grep CATTLE_RANCHER_SERVER_IMAGE Dockerfile | awk '{print $3}')}
IMAGE=rancher/server:${TAG}

docker build -t ${IMAGE} .

echo Done building ${IMAGE}
