# Copyright © 2018 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: BSD-2-Clause

: ${TAG:=latest}
: ${IMAGE:=vmware/kube-fluentd-operator}

for a in $@; do
  if [[ ! -d "$a" ]]; then
    echo "error: $a is not a directory"
    exit 1
  fi

  p="$(realpath "$a")"
  docker run --entrypoint=/bin/validate-from-dir.sh \
    --net=host \
    --rm \
    -v "$p:$p:ro" \
    -e "DATASOURCE_DIR=$p" \
    $IMAGE:$TAG
done