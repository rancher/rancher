#!/bin/bash
set -e -x

cd "$(dirname "$0")"

if [ ! -e target/.done ]; then
    mkdir -p target
    S6_BUILDER=$(cat /dev/urandom | env LC_CTYPE=C tr -cd 'a-f0-9' | head -c 16)
    trap "docker rm -fv ${S6_BUILDER} 2>/dev/null || :" EXIT
    docker run --name=${S6_BUILDER} rancher/s6-builder:v2.2.4.3 /opt/build.sh
    docker cp ${S6_BUILDER}:/output/s6-static.tar.gz ./target/
    touch target/.done
fi

TAG=${TAG:-$(awk '/CATTLE_RANCHER_SERVER_IMAGE/{print $3}' Dockerfile)}
IMAGE=rancher/server:${TAG}

docker build -t "${IMAGE}" .

echo Done building "${IMAGE}"
