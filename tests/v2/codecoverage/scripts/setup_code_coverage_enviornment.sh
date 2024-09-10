#!/bin/bash
set -e
cd $(dirname $0)/../../../../

echo "building bins"
sh tests/v2/codecoverage/scripts/build_code_coverage_images.sh

echo "build corral packages"
sh tests/v2/codecoverage/scripts/build_corral_packages.sh

echo "running rancher corral"
tests/v2/codecoverage/bin/ranchercorral

echo "setup rancher"
tests/v2/codecoverage/bin/setuprancher