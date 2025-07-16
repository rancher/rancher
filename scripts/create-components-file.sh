#!/bin/bash
# This script will create a txt file with -rc images/components which will be used as (pre) release description by Drone
set -e -x

echo "Creating ./bin/rancher-components.txt"

cd $(dirname $0)/..

mkdir -p bin

COMPONENTSFILE=./bin/rancher-components.txt
K8SVERSIONSFILE=./bin/rancher-rke-k8s-versions.txt

echo "# Images with -rc" > $COMPONENTSFILE

# find any images that contain `-rc` or `-alpha`
grep -h -e '-rc' -e '-alpha' ./bin/rancher-images.txt ./bin/rancher-windows-images.txt | sort -u >>$COMPONENTSFILE
echo "" >>$COMPONENTSFILE

echo "# Components with -rc" >> $COMPONENTSFILE

# find components envs that match `ENV CATTLE_*_VERSION`, NOT contain `http` or `$` or `MIN_VERSION` and contain `-rc` or `-alpha`
grep -e '^ENV CATTLE_.*_VERSION.*' package/Dockerfile | grep -E -v 'http|\$|MIN_VERSION' | grep -e '-rc' -e '-alpha' | sed 's/ENV CATTLE_//g' | sed 's/=/ /g' | sort >>$COMPONENTSFILE
# find deps that contain `rancher/` and `-rc` or `-alpha`, NOT contain `./` or `/pkg/apis` or `/pkg/client` or `=>` or start with `module`
grep -e "rancher/" go.mod | grep -v -e '\./' -e '/pkg/apis' -e '/pkg/client' -e '^module' -e '=>' | grep -e '-rc' -e '-alpha' | cut -d '/' -f3 | awk '$1 = toupper($1)' | sort >>$COMPONENTSFILE
echo "" >>$COMPONENTSFILE

echo "# Min version components with -rc" >> $COMPONENTSFILE

# find components envs that match `ENV CATTLE_*_MIN_VERSION` and contain `-rc` or `-alpha`
grep -e '^ENV CATTLE_.*_MIN_VERSION.*' package/Dockerfile | grep -e '-rc' -e '-alpha' | sed 's/ENV CATTLE_//g' | sort >>$COMPONENTSFILE
echo "" >>$COMPONENTSFILE

if [[ -f "$K8SVERSIONSFILE" ]]; then
    echo "# RKE Kubernetes versions" >> $COMPONENTSFILE
    cat $K8SVERSIONSFILE >> $COMPONENTSFILE
fi

echo "# Chart/KDM sources" >> $COMPONENTSFILE

bash ./scripts/check-chart-kdm-source-values >> $COMPONENTSFILE

echo "Done creating ./bin/rancher-components.txt"
