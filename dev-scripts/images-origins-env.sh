#!/usr/bin/env sh

SYSTEM_AGENT_UPGRADE_TAG=$(grep "ENV CATTLE_SYSTEM_AGENT_VERSION" package/Dockerfile | awk -F'=' '{ print $NF }')-suc
WINS_AGENT_UPGRADE_TAG=$(grep "ENV CATTLE_WINS_AGENT_VERSION" package/Dockerfile | awk -F'=' '{ print $NF }')

# Query KDM data for RKE2 released versions where server args are defined.
RKE2_RELEASE_VERSIONS=$(jq -r '[.rke2.releases[] | select(.serverArgs) | .version] | join(" ")' bin/data.json)
# Convert versions with build metadata into valid image tags (replace + for -) and construct an array of tags.
RKE2_RELEASE_TAGS=( $(echo $RKE2_RELEASE_VERSIONS | tr + -) )

# Query KDM data for K3S released versions where server args are defined.
K3S_RELEASE_VERSIONS=$(jq -r '[.k3s.releases[] | select(.serverArgs) | .version] | join(" ")' bin/data.json)
# Convert versions with build metadata into valid image tags (replace + for -) and construct an array of tags.
K3S_RELEASE_TAGS=( $(echo $K3S_RELEASE_VERSIONS | tr + -) )

# Prefix image repo and name to tags.
export SYSTEM_AGENT_INSTALLER_RKE2_IMAGES=( "${RKE2_RELEASE_TAGS[@]/#/${REPO}/system-agent-installer-rke2:}" )
export SYSTEM_AGENT_INSTALLER_K3S_IMAGES=( "${K3S_RELEASE_TAGS[@]/#/${REPO}/system-agent-installer-k3s:}" )
export SYSTEM_AGENT_UPGRADE_IMAGE=${REPO}/system-agent:${SYSTEM_AGENT_UPGRADE_TAG}

export WINS_AGENT_UPGRADE_IMAGE=${REPO}/wins:${WINS_AGENT_UPGRADE_TAG}
export CHART_REPO_DIR=build/charts
