#!/bin/bash

ID=$(grep :devices: /proc/self/cgroup | head -n1 | awk -F/ '{print $NF}' | sed -e 's/docker-\(.*\)\.scope/\1/')
IMAGE=$(docker inspect -f '{{.Config.Image}}' $ID)

docker run --privileged --net host --pid host -v /:/host --rm --entrypoint /usr/bin/share-mnt $IMAGE "$@" -- norun
while ! docker start kubelet; do
    sleep 2
done
docker kill $ID
