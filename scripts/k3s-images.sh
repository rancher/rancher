#!/bin/bash
set -e -x

images=$(grep -e 'docker.io/rancher/pause' -e 'docker.io/rancher/coredns-coredns' /usr/tmp/k3s-images.txt)
xargs -n1 docker pull <<< "${images}"
docker save -o /go/src/github.com/rancher/rancher/package/k3s-airgap-images.tar ${images}
