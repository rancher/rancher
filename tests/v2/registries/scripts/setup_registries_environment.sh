#!/bin/bash
set -ex
cd $(dirname $0)/../../../../

echo "building bins"
sh tests/v2/registries/scripts/build_registries_images.sh

echo "build corral packages"
sh tests/v2/registries/scripts/build_corral_packages.sh

echo | corral config

echo "creating registry auth disabled bin"
tests/v2/registries/bin/registryauthdisabled

sleep 1

echo "building registry auth enabled bin"
tests/v2/registries/bin/registryauthenabled

sleep 1

corral list

echo "running rancher corral"
tests/v2/registries/bin/ranchercorral 


echo "setup rancher"
tests/v2/registries/bin/setuprancher
