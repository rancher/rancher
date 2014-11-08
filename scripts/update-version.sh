#!/bin/bash

cd $(dirname $0)/..

VER=$1

sed -i 's!FROM.*!FROM cattle/server:'$VER'!' server/Dockerfile
sed -i 's!ENV CATTLE_AGENT_INSTANCE_IMAGE .*!ENV CATTLE_AGENT_INSTANCE_IMAGE cattle/agent-instance:'$VER'!' server/Dockerfile
sed -i 's!FROM.*!FROM cattle/agent:'$VER'!' agent/Dockerfile

echo $VER > version
