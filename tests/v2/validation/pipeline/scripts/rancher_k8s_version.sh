#!/bin/bash

latest_rancher_version=$(curl -s https://prime.ribs.rancher.io/index.html | grep  "release-v[0-9]*" | sort -V | tail -n 1 |  sed -E 's/.*release-(v[0-9]+\.[0-9]+\.[0-9]+).*/\1/')
echo "Latest Rancher Version: $latest_rancher_version"

k8s_versions=$(curl -s "https://prime.ribs.rancher.io/rancher/${latest_rancher_version}/rancher-images.txt" | grep "rancher/hardened-kubernetes:" | awk -F':' '{print $2}'  | awk -F '-build' '{print $1}' | sort -V | awk -F '.' '
        {versions[$1"."$2] = $0} 
        END {
            for (v in versions) print versions[v]""
        }' | sort -V | tail -n 2)


# Output the results
echo "Latest Kubernetes Versions: "
echo "$k8s_versions" 

