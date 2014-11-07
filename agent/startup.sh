#!/bin/bash

CATTLE_AGENT_IMAGE=${CATTLE_AGENT_IMAGE:-rancher/agent:latest}

load()
{
    CONTENT=$(curl -sL $URL)

    if [[ "$CONTENT" =~ .!/bin/sh.* ]]; then
        eval "$CONTENT"
    fi
}

if [ "$1" == "--" ]; then
    shift 1
    exec "$@"
fi

if [ "$1" = "" ]; then
    echo URL is required as a parameter 1>&2
    exit 1
fi

URL=$1

if ! curl -sL $URL >/dev/null 2>&1; then
    echo "Invalid URL [$URL] or not authorized"
    exit 1
fi

load

if [ -z "$CATTLE_REGISTRATION_SECRET_KEY" ]; then
    URL=$(./resolve_url.py $URL)
    load
fi

if [ -z "$CATTLE_REGISTRATION_SECRET_KEY" ]; then
    echo "Failed to load environment" 1>&2
    exit 1
fi

export CATTLE_AGENT_IP=${CATTLE_AGENT_IP:-$DETECTED_CATTLE_AGENT_IP}

while docker inspect rancher-agent >/dev/null 2>&1; do
    docker rm -f rancher-agent
    sleep 1
done

docker run \
    --net=host \
    --restart=always \
    --privileged \
    --name rancher-agent \
    --privileged \
    -e CATTLE_EXEC_AGENT=true \
    -e CATTLE_REGISTRATION_ACCESS_KEY="${CATTLE_REGISTRATION_ACCESS_KEY}" \
    -e CATTLE_REGISTRATION_SECRET_KEY="${CATTLE_REGISTRATION_SECRET_KEY}" \
    -e CATTLE_AGENT_IP="${CATTLE_AGENT_IP}" \
    -e CATTLE_URL="${CATTLE_URL}" \
    -v /lib/modules:/host/lib/modules:ro \
    -v /var/lib/docker:/host/var/lib/docker \
    -v /var/lib/cattle:/host/var/lib/cattle \
    -v /opt/bin:/host/opt/bin \
    -v /proc:/host/proc \
    -v /run:/host/run \
    -d \
    "${CATTLE_AGENT_IMAGE}" -- /agent-env.sh
