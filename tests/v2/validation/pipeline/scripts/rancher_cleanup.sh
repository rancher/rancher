#!/bin/bash
set -ex
cd $(dirname $0)/../../../../../

echo | corral config

corral list

echo "cleanup rancher"
tests/v2/validation/registries/bin/ranchercleanup
