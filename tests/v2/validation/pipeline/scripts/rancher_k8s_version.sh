#!/bin/bash

latest_rancher_version=$(curl -s https://prime.ribs.rancher.io/index.html | grep  "release-v[0-9]*" | sort -V | tail -n 1 |  sed -E 's/.*release-(v[0-9]+\.[0-9]+\.[0-9]+).*/\1/')
echo "Latest Rancher Version: $latest_rancher_version"

rke2_k8s_versions=$(curl -s "https://prime.ribs.rancher.io/rancher/${latest_rancher_version}/rancher-images.txt" | grep "rancher/hardened-kubernetes:" | awk -F':' '{print $2}'  | awk -F '-build' '{print $1}' | sort -V | awk -F '.' '
        {versions[$1"."$2] = $0} 
        END {
            for (v in versions) print versions[v]""
        }' | sed 's/-rke2/+rke2/'| sort -V | tail -n 2)

k3s_k8s_versions=$(curl -s "https://prime.ribs.rancher.io/rancher/${latest_rancher_version}/rancher-images.txt" | grep "rancher/k3s-upgrade:" | awk -F':' '{print $2}'  | awk -F '-build' '{print $1}' | sort -V | awk -F '.' '
        {versions[$1"."$2] = $0} 
        END {
            for (v in versions) print versions[v]""
        }' | sed 's/-k3s/+k3s/' | sort -V | tail -n 2)

rke1_k8s_versions=$(curl -s "https://prime.ribs.rancher.io/rancher/${latest_rancher_version}/rancher-rke-k8s-versions.txt"  | sort -V | tail -n 2)

echo "Latest Kubernetes Versions: "
echo -e "$rke2_k8s_versions\n$k3s_k8s_versions\n$rke1_k8s_versions"


