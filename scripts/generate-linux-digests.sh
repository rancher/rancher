#!/usr/bin/env bash
cd $(dirname $0)/.. 
mkdir -p bin

declare -a archs=(amd64 arm64 s390x)

if [ -z "${DRONE_TAG}" ]; then
    echo "Environment variable DRONE_TAG not set"
    exit 0
fi

DIGEST_TEMPLATE_FILENAME="./bin/rancher-images-digests-linux"
IMAGES_FILE=$(mktemp)
IMAGES_URL="https://github.com/rancher/rancher/releases/download/${DRONE_TAG}/rancher-images.txt"

wget --retry-connrefused --waitretry=1 --read-timeout=20 --timeout=15 -t 10 $IMAGES_URL -O $IMAGES_FILE

for image in $(cat $IMAGES_FILE); do
    INSPECT_JSON=$(skopeo inspect "docker://${image}" --raw)
    MEDIATYPE=$(echo "${INSPECT_JSON}" | jq -r .mediaType)
    echo "Image: ${image}, mediaType: ${MEDIATYPE}"
    if [ "${MEDIATYPE}" = "application/vnd.docker.distribution.manifest.list.v2+json" ] || [ "${MEDIATYPE}" = "application/vnd.oci.image.index.v1+json" ]; then
        DIGEST=$(echo -n "${INSPECT_JSON}" | sha256sum | awk '{ print $1 }')
        for arch in "${archs[@]}"; do
            if echo "${INSPECT_JSON}" | jq -e  --arg ARCH "$arch" 'select(.manifests[].platform.architecture == $ARCH)' >/dev/null 2>&1; then
                echo "docker.io/${image} sha256:${DIGEST}" >> "${DIGEST_TEMPLATE_FILENAME}-${arch}.txt"
            fi
        done
    else
        for arch in "${archs[@]}"; do
            INSPECT_JSON=$(skopeo --override-arch $arch inspect "docker://${image}")
            if echo "${INSPECT_JSON}" | jq -e --arg ARCH "$arch" '.Architecture == $ARCH' >/dev/null 2>&1; then
                echo "Image: ${image}, arch ${arch}, FOUND"
                DIGEST=$(skopeo --override-arch $arch inspect "docker://${image}" --raw | sha256sum | awk '{ print $1 }')
                echo "docker.io/${image} sha256:${DIGEST}" >> "${DIGEST_TEMPLATE_FILENAME}-${arch}.txt"
            else
                echo "Image: ${image}, arch ${arch}, NOT_FOUND"
            fi
        done
    fi
done

rm -f ${IMAGES_FILE}

ls -la ./bin/rancher-images-digests-linux-*
