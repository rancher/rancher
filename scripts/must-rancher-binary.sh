#!/bin/sh

# Ensures that the rancher binary exists, if it doesn't build or pull from a preloaded image

if [ -f "bin/rancher" ];then
  echo "binary found"
  exit 0
fi

if [ -z "${TAG}" ]; then
  >&2 echo "error: missing TAG var"
  exit 1
fi

if ! docker image inspect "rancher/rancher:$TAG" >/dev/null 2>&1; then
    echo "building rancher from source - no preloaded container image available"
    (
      cd "$(dirname "$0")/.." || exit
      make quick-binary-server
    )
else
    # otherwise just copy it from the artifacts that are already there. neat!
    echo "pulling bin/rancher from preloaded container image"
    container_id=$(docker create "rancher/rancher:$TAG")
    docker cp "$container_id:/usr/bin/rancher" bin/rancher
    docker rm "$container_id"
fi
