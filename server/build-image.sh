#!/bin/bash

TAG=${TAG:-dev}
IMAGE=rancher/server:${TAG}

cd $(dirname $0)
docker build -t ${IMAGE} .
