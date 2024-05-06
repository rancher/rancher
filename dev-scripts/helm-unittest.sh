#!/bin/bash

# note: requires helm-unittest (https://github.com/helm-unittest/helm-unittest) to be installed beforehand
# run in root of rancher - i.e. bash dev-scripts/helm-unittest.sh
# change automated parts of templates
test_image="rancher/rancher"
test_image_tag="v2.7.0"
sed -i -e "s/%VERSION%/${test_image_tag}/g" ./chart/Chart.yaml
sed -i -e "s/%APP_VERSION%/${test_image_tag}/g" ./chart/Chart.yaml
sed -i -e "s@%POST_DELETE_IMAGE_NAME%@${test_image}@g" ./chart/values.yaml
sed -i -e "s/%POST_DELETE_IMAGE_TAG%/${test_image_tag}/g" ./chart/values.yaml

# test - need to be in the chart directory during the test so it can find Chart.yaml
cd chart
helm lint ./
helm unittest ./
cd ..

# clean
git checkout chart/Chart.yaml chart/values.yaml
