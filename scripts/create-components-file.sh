#!/bin/bash
# This script will create a txt file with -rc images/components which will be used as (pre) release description by Drone
set -e -x

echo "Creating ./bin/rancher-components.txt"

cd $(dirname $0)/..

mkdir -p bin

COMPONENTSFILE=./bin/rancher-components.txt

echo "# Images with -rc" > $COMPONENTSFILE

printf '%s\n' "$(grep -h "\-rc" ./bin/rancher-images.txt ./bin/rancher-windows-images.txt | awk -F: '{ print $1,$2 }')" | sort -u >> $COMPONENTSFILE

echo "# Components with -rc" >> $COMPONENTSFILE

printf '%s\n' "$(grep "_VERSION" ./package/Dockerfile | grep ENV | egrep -v "http|\\$|MIN_VERSION" | grep CATTLE |sed 's/CATTLE_//g' | sed 's/=/ /g' |  awk '{ print $2,$3 }' | sort | grep "\-rc")" >> $COMPONENTSFILE

printf '%s\n' "$(grep "rancher/" ./go.mod | egrep -v "\./"  | egrep -v "\/pkg\/apis|\/pkg\/client|^module" | grep -v "=>" | awk -F'/' '{ print $NF }' | awk '$1 = toupper($1)' | sort | grep "\-rc")" >> $COMPONENTSFILE

K8SVERSIONSFILE=./bin/rancher-rke-k8s-versions.txt

if [[ -f "$K8SVERSIONSFILE" ]]; then
    echo "# RKE Kubernetes versions" >> $COMPONENTSFILE
    cat $K8SVERSIONSFILE >> $COMPONENTSFILE
fi

echo "Done creating ./bin/rancher-components.txt"
