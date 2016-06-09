#!/bin/bash
set -e -x

ARCH=${ARCH:-"$(docker version -f {{.Server.Arch}})"}
IMAGE=${IMAGE:-"$(grep 'ENV RANCHER_AGENT_IMAGE ' Dockerfile | awk '{print $3}')"}

cd $(dirname $0)
cat > image-manifest.yml << EOF
image: ${IMAGE}
manifests:
- image: ${IMAGE}_amd64
  platform:
    architecture: amd64
    os: linux
EOF

FROM_IMAGE=$(grep -e '^FROM ' Dockerfile | awk '{print $2}')
ARCH_IMAGES=($(grep -e '^# FROM ' Dockerfile | head -n 1 | sed 's/# FROM //'))
for s in "${ARCH_IMAGES[@]}"; do
    a="$(echo ${s} | cut -f1 -d=)"
    if [ "${a}" = "${ARCH}" ]; then
        FROM_ARCH_IMAGE="$(echo ${s} | cut -f2 -d=)"
    fi
    cat >> image-manifest.yml << EOF
- image: ${IMAGE}_${a}
  platform:
    architecture: ${a}
    os: linux
EOF
done
if [ "${FROM_ARCH_IMAGE}" != "" ]; then
    docker inspect ${FROM_ARCH_IMAGE} >/dev/null 2>&1 || docker pull ${FROM_ARCH_IMAGE}
    docker tag ${FROM_ARCH_IMAGE} ${FROM_IMAGE}
fi

cat Dockerfile | sed "s/^# ARCH=${ARCH}: //" > Dockerfile.tmp
trap 'rm Dockerfile.tmp' EXIT

docker build -t ${IMAGE}_${ARCH} -f Dockerfile.tmp .
docker tag ${IMAGE}_${ARCH} ${IMAGE}

if [ -n "$IMAGE_REGISTRY" ]; then
    echo $IMAGE >> ${IMAGE_REGISTRY}
fi
