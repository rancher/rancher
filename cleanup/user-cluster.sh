#!/bin/bash
# set -x
set -e

# Location of the yaml to use to deploy the cleanup job
yaml_url=https://raw.githubusercontent.com/rancher/rancher/master/cleanup/user-cluster.yml

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

# Pull the yaml and replace the agent_image holder with the passed in image
yaml=$(curl --insecure -sfL $yaml_url | sed -e 's=agent_image='"$agent_image"'=')

if [ "$2" = "-dry-run" ]
then
    # Uncomment the env var for dry-run mode
    yaml=$(sed -e 's/# // ' <<< "$yaml")
fi

echo "$yaml" | kubectl --kubeconfig ~/development/kube_config_cluster.yml apply -f -

# Get the pod ID to tail the logs
pod_id=$(kubectl --kubeconfig ~/development/kube_config_cluster.yml get pod -l job-name=cattle-cleanup-job -o jsonpath="{.items[0].metadata.name}")

declare -i count=0
until kubectl --kubeconfig ~/development/kube_config_cluster.yml logs $pod_id -f
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
echo "$yaml" | kubectl --kubeconfig ~/development/kube_config_cluster.yml delete -f -
