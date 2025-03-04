#!/bin/bash

# note: requires helm-unittest (https://github.com/helm-unittest/helm-unittest) to be installed beforehand
# run in root of rancher - i.e. bash dev-scripts/helm-unittest.sh
# change automated parts of templates

source $(dirname $0)/prepare-chart

# test - need to be in the chart directory during the test so it can find Chart.yaml
cd chart
helm lint ./
helm unittest ./
cd ..

# clean
git checkout chart/Chart.yaml chart/values.yaml
