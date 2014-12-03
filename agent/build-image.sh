#!/bin/bash

TAG=${TAG:-dev}
IMAGE=rancher/agent:${TAG}

cd $(dirname $0)
docker build -t ${IMAGE} .
