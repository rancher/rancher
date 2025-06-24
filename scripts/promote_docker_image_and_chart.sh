#!/bin/bash

drone_cli_help()
{
    echo "  Requires Drone CLI and personal access token"
    echo "  Download CLI following instructions at https://docs.drone.io/cli/install/"
    echo "  Then configure access token: https://docs.drone.io/cli/setup/"
    echo ""
    echo "  To test if Drone CLI is properly configured type:"
    echo ""
    echo "     drone build ls rancher/rancher"
    echo ""
    echo "  This will show the last 25 builds"
}

if [[ $# -ne 2 ]]; then
    echo "Promote Chart and Docker image to stable or latest tag"
    echo "  $0 <tag> <stable_or_latest>"
    echo ""
    drone_cli_help
    exit 1
fi

if ! drone -v; then
    drone_cli_help
    exit 1
fi

source_tag=$1
destination_tag=$2

page_limit=100

if [[ ! $destination_tag =~ ^(stable|latest|donotuse)$ ]]; then
  echo "Docker tag needs to be stable or latest (or donotuse for testing), not ${destination_tag}"
  exit 1
fi

echo "Promoting Docker Image ${source_tag} to ${destination_tag}"

page=1
until [ $page -gt $page_limit ]; do
  echo "Finding build number for tag ${source_tag}"
  build_number=$(drone build ls rancher/rancher --page $page --event tag --format "{{.Number}},{{.Ref}}"| grep ${source_tag}$ |cut -d',' -f1|head -1)
  if [[ -n ${build_number} ]]; then
    echo "Found build number ${build_number} for tag ${source_tag}"
    drone build promote rancher/rancher ${build_number} promote-docker-image --param=SOURCE_TAG=$source_tag --param=DESTINATION_TAG=$destination_tag
    exit 0
    break
  fi
  ((page++))
  sleep 1
done

echo "No build number found for docker image tag: ${source_tag}"
echo "Promoting Chart ${source_tag} to ${destination_tag}"

page=1
until [ $page -gt $page_limit ]; do
  echo "Finding build number for tag ${source_tag}"
  build_number=$(drone build ls rancher/rancher --event tag --format "{{.Number}},{{.Ref}}"| grep ${1}$ |cut -d',' -f1|head -1)
  if [[ -n ${build_number} ]];then
    echo "Found build number ${build_number} for tag ${source_tag}"
    drone build promote rancher/rancher ${build_number} promote-stable
    exit 0
  fi
  ((page++))
  sleep 1
done

echo "No Build Found for TAG: ${1}"

exit 1
 