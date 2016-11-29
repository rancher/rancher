#!/bin/bash

REPO=${REPO:-rancher}
TAG=${TAG:-dev}

docker build -t $REPO/agent-base:${TAG} .
echo Built $REPO/agent-base:${TAG}
