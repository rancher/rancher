#!/bin/bash
set -x

trap 'exit 0' SIGTERM
ID=$(grep :devices: /proc/self/cgroup | head -n1 | awk -F/ '{print $NF}' | sed -e 's/docker-\(.*\)\.scope/\1/')
bash -c "$1"

docker kill --signal=SIGTERM $ID