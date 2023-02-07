#!/bin/bash
set -ex
cd $(dirname $0)/../../../../

echo "build corral packages"
sh tests/v2/validation/scripts/build_corral_packages.sh

echo | corral config

echo "build registries images"
sh tests/v2/validation/scripts/build_registries_images.sh

corral list

echo "running rancher corral"
tests/v2/validation/registries/bin/rancherha
