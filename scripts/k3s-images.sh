#!/bin/bash
set -e -x

cd $(dirname $0)/..

mkdir -p bin

images=$(grep -e 'docker.io/rancher/pause' -e 'docker.io/rancher/coredns-coredns' /usr/tmp/k3s-images.txt)
xargs -n1 docker pull <<< "${images}"
docker save -o ./bin/k3s-airgap-images.tar ${images}
