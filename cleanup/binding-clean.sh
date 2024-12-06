#!/bin/bash
# set -x
set -e

CLEAR='\033[0m'
RED='\033[0;31m'

# Location of the yaml to use to deploy the cleanup job
yaml_url=https://raw.githubusercontent.com/rancher/rancher/release/v2.9/cleanup/binding-clean.yaml

# 120 is equal to a minute as the sleep is half a second
timeout=120

# Agent image to use in the yaml file
agent_image="$1"

show_usage() {
  if [ -n "$1" ]; then
    echo -e "${RED}👉 $1${CLEAR}\n";
  fi
	echo -e "Usage: $0 [AGENT_IMAGE] [FLAGS]"
	echo "AGENT_IMAGE is a required argument"
	echo ""
	echo "Flags:"
	echo -e "\t-dry-run Display the resources that would will be updated without making changes"
}

if [ $# -lt 1 ]
then
	show_usage "AGENT_IMAGE is a required argument"
	exit 1
fi

if [[ $1 == "-h" ||$1 == "--help" ]]
then
	show_usage
	exit 0
fi

# Pull the yaml and replace the agent_image holder with the passed in image
yaml=$(curl --insecure -sfL $yaml_url | sed -e 's=agent_image='"$agent_image"'=')

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

# Cleanup after it completes successfully
echo "$yaml" | kubectl delete -f -
