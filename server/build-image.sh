#!/bin/bash
set -e

build_from_source_image() {
    local branch=$1
    local version=$2

    cat > Dockerfile.$version << EOF
FROM ${IMAGE}
ENV CATTLE_BRANCH origin/$branch
EOF

    docker build -t "${REPO}:$version" -f Dockerfile.$version .
    rm Dockerfile.$version
}

cd "$(dirname "$0")"

if [ ! -e target/.done ]; then
    mkdir -p target
    curl -sL -o target/s6-overlay-amd64-static.tar.gz https://github.com/just-containers/s6-overlay/releases/download/v1.19.1.1/s6-overlay-amd64.tar.gz
    touch target/.done
fi

TAG=${TAG:-$(awk '/ENV CATTLE_RANCHER_SERVER_VERSION/{print $3}' Dockerfile)}
REPO=${REPO:-$(awk '/ENV CATTLE_RANCHER_SERVER_IMAGE/{print $3}' Dockerfile)}
IMAGE=${REPO}:${TAG}

docker build -t "${IMAGE}" .

build_from_source_image master master
build_from_source_image v1.6 v1.6-dev

echo -e "Done building:\n ${REPO}:master\n ${REPO}:v1.6-dev\n ${IMAGE}"
