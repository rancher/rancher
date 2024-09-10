#!/bin/bash
set -e

cd $(dirname $0)

echo "build rancher"
./build-rancher.sh
echo "build agent"
./build-agent.sh