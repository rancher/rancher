#!/bin/bash
set -e
VERSION=$SYSTEM_AGENT_VERSION

updated_install_script_url="https://github.com/rancher/system-agent/releases/download/${VERSION}/install.sh" # update it to regular expression
existing_install_script_url=$(grep -E -o 'https:\/\/github.com\/rancher\/system-agent\/releases\/download\/[^/]+\/install.sh' pkg/settings/setting.go)
sed -i "s|$existing_install_script_url|$updated_install_script_url|g" pkg/settings/setting.go

sed -i "s|^ENV CATTLE_SYSTEM_AGENT_VERSION .\+$|ENV CATTLE_SYSTEM_AGENT_VERSION ${VERSION}|g" package/Dockerfile # try to use regex to avoid extracting the string from the file
sed -i "s|^ENV CATTLE_SYSTEM_AGENT_VERSION .\+$|ENV CATTLE_SYSTEM_AGENT_VERSION ${VERSION}|g" tests/v2/codecoverage/package/Dockerfile # try to use regex to avoid extracting the string from the file
