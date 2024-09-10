#!/bin/bash
set -ex
cd $(dirname $0)/../../../../../

echo "build corral packages"
sh tests/v2/validation/pipeline/scripts/build_corral_packages.sh

echo | corral config

echo "build rancherHA images"
sh tests/v2/validation/pipeline/scripts/build_rancherha_images.sh

corral list

echo "running rancher corral"
tests/v2/validation/registries/bin/rancherha
