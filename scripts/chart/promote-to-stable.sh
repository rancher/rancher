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

if [[ -z "${1}" ]]; then
    echo "Promote a chart in latest to stable."
    echo "  $0 <tag>"
    echo ""
    drone_cli_help
    exit 1
fi

if ! drone -v; then
    drone_cli_help
    exit 1
fi

source_tag=$1
page=1
until [ $page -gt 100 ]; do
  echo "Finding build number for tag ${source_tag}"
  build_number=$(drone build ls rancher/rancher --page $page --event tag --format "{{.Number}},{{.Ref}}"| grep "${source_tag}"$ |cut -d',' -f1|head -1)
  if [[ -n ${build_number} ]]; then
    echo "Found build number ${build_number} for tag ${source_tag}"
    drone build promote rancher/rancher "${build_number}" promote-stable
    exit 0
    break
  fi
  ((page++))
  sleep 1
done

echo "No Build Found for TAG: ${1}"
exit 1
