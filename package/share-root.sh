#!/bin/bash
set -x

trap 'exit 0' SIGTERM
ID=$(grep :devices: /proc/self/cgroup | head -n1 | awk -F/ '{print $NF}' | sed -e 's/docker-\(.*\)\.scope/\1/')
IMAGE=$(docker inspect -f '{{.Config.Image}}' $ID)
bash -c "$1"

docker run --privileged --net host --pid host -v /:/host --rm --entrypoint /usr/bin/share-mnt $IMAGE "${@:2}" -- norun
while ! docker start kubelet; do
    sleep 2
done
docker kill --signal=SIGTERM $ID