#!/bin/bash
# set -x
set -e

CLEANUP_YAML_PATH="/cleanup/user-cluster.yml"
BASE_URL="https://raw.githubusercontent.com/rancher/rancher/"

# 120 is equal to a minute as the sleep is half a second
timeout=120

# Agent image to use in the yaml file
agent_image="$1"

show_usage() {
	echo -e "Usage: $0 [AGENT_IMAGE] [FLAGS]"
	echo "AGENT_IMAGE is a required argument"
	echo ""
	echo "Flags:"
	echo -e "\t-dry-run Display the resources that would will be updated without making changes"
}

if [ $# -lt 1 ]
then
	show_usage
	exit 1
fi

if [[ $1 == "-h" ||$1 == "--help" ]]
then
	show_usage
	exit 0
fi

if ! [[ "$agent_image" == *rancher/rancher-agent:* ]]; then
    echo "ERROR: Invalid agent image format: $agent_image"
    echo "Expected format: rancher/rancher-agent:vX.YY"
    exit 1
fi

AGENT_TAG="${agent_image##*:}"

if [ -z "$AGENT_TAG" ] || [[ ! "$AGENT_TAG" =~ ^v[0-9]+\.[0-9]+ ]]; then
    echo "Error: Could not extract a valid version tag (vX.Y format) from the AGENT_IMAGE '$agent_image'."
    exit 1
fi

MINOR_VERSION=$(echo "$AGENT_TAG" | awk -F'-' '{print $1}' | awk -F'.' '{print $1"."$2}')
YAML_BRANCH_PATH="release/${MINOR_VERSION}"

if [ -z "$MINOR_VERSION" ]; then
    echo "Error: Could not determine minor version from tag '$AGENT_TAG'.Using main instead."
    YAML_BRANCH_PATH="main"
fi

# Location of the yaml to use to deploy the cleanup job
yaml_url="${BASE_URL}${YAML_BRANCH_PATH}${CLEANUP_YAML_PATH}"

echo "Using Minor Version Path: ${YAML_BRANCH_PATH}"
echo "Cleanup YAML URL: ${yaml_url}"

# Pull the yaml and replace the agent_image holder with the passed in image
yaml=$(curl --insecure -sfL $yaml_url | sed -e 's=agent_image='"$agent_image"'=')

if [ $? -ne 0 ]; then
    echo "Error: Failed to download cleanup YAML from $yaml_url"
    echo "Check if the version path '$YAML_BRANCH_PATH' exists in the rancher/rancher repository."
    exit 1
fi

if [ "$2" = "-dry-run" ]
then
    # Uncomment the env var for dry-run mode
    yaml=$(sed -e 's/# // ' <<< "$yaml")
fi

echo "$yaml" | kubectl apply -f -

# Get the pod ID to tail the logs
pod_id=$(kubectl get pod -l job-name=cattle-cleanup-job -o jsonpath="{.items[0].metadata.name}")

declare -i count=0
until kubectl logs $pod_id -f
do
    if [ $count -gt $timeout ]
    then
        echo "Timout reached, check the job by running kubectl get jobs"
        exit 1
    fi
    sleep 0.5
    count+=1
done

echo "Cleanup job finished successfully. Deleting job resources..."
# Cleanup after it completes successfully
echo "$yaml" | kubectl delete -f -
