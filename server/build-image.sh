#!/bin/bash
set -e -x

cd $(dirname $0)

mkdir -p target
docker run -it -v $(pwd)/target:/output rancher/s6-builder:v0.1.0 /opt/build.sh

TAG=${TAG:-$(grep CATTLE_RANCHER_SERVER_IMAGE Dockerfile | awk '{print $3}')}
IMAGE=rancher/server:${TAG}

docker build -t ${IMAGE} .

echo Done building ${IMAGE}
