###############################################################
# TDOO: I have done NOTHING up to now for debugging the agent !
###############################################################

#!/bin/bash
set -e

source $(dirname $0)/version

ARCH=${ARCH:-"amd64"}
SUFFIX=""
[ "${ARCH}" != "amd64" ] && SUFFIX="_${ARCH}"

cd $(dirname $0)/../package/debugger

TAG=${TAG:-${VERSION}${SUFFIX}}
REPO=${REPO:-rancher}

#if echo $TAG | grep -q dirty; then
    TAG=debug
#fi

#if [ -n "$DRONE_TAG" ]; then
#    TAG=$DRONE_TAG
#fi

cp ../../bin/agent .

IMAGE=${REPO}/rancher:${TAG}
AGENT_IMAGE=${REPO}/rancher-agent:${TAG}
docker build --build-arg VERSION=${TAG} -t ${IMAGE} -f Dockerfile.debugger .
docker build --build-arg VERSION=${TAG} -t ${AGENT_IMAGE} -f Dockerfile.agent .
echo ${IMAGE} > ../../dist/images
echo ${AGENT_IMAGE} >> ../../dist/images
echo Built ${IMAGE} #${AGENT_IMAGE}
echo

cd ../../bin
go run ../pkg/image/export/main.go $IMAGE $AGENT_IMAGE

