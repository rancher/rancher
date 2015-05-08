#!/bin/bash


cd $(dirname $0)

mkdir -p target
docker run -it -v $(pwd)/target:/output rancher/s6-builder:v0.1.0 /opt/build.sh

TAG=${TAG:-dev}
IMAGE=rancher/server:${TAG}

docker build -t ${IMAGE} .
