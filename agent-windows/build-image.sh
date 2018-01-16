#!/bin/bash

cd $(dirname $0)

cleanup(){
    rm ./tools/devcon.exe
}

trap cleanup INT
curl -L https://github.com/rancher/windows-binaries/releases/download/v0.0.1/devcon.exe > ./tools/devcon.exe
MD5=$(wget -q -O- https://github.com/rancher/windows-binaries/releases/download/v0.0.1/MD5SUM)
MD5CHECK=$(md5sum devcon.exe  | awk '{print $1}')
if [ $MD5 -ne $MD5CHECK ]; then
    echo "download devcon.exe error, md5 not match"
fi

if [ -z "$IMAGE" ]; then
    IMAGE=$(grep RANCHER_AGENT_WINDOWS_IMAGE Dockerfile | awk '{print $3}')
fi

echo Building $IMAGE
docker build -t ${IMAGE} .

cleanup