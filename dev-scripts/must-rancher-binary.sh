#!/bin/sh

if [ -z "${TAG}" ]; then
  >&2 echo "error: missing TAG var"
  exit 1
fi

if ! docker image inspect "rancher/rancher:$TAG" >/dev/null 2>&1; then
  echo "building rancher from source - no preloaded container image available"
  make -C .. quick-binary-server
else
  # otherwise just copy it from the artifacts that are already there. neat!
  echo "pulling bin/rancher from preloaded container image"
  container_id=$(docker create "rancher/rancher:$TAG")
  docker cp "$container_id:/usr/bin/rancher" bin/rancher
  docker rm "$container_id"
fi
