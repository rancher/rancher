#!/bin/bash

drone_cli_help() {
    echo "Promotes build to promote-docker-image"
    echo "and to promote-stable if destination is stable."
    echo "Usage: $0 [-h] [-n] <tag> <destination_tag>"
    echo "  -h  Show help information."
    echo "  -n  Perform a dry run without making any changes."
    echo ""
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

drone_server="https://drone-publish.rancher.io"
dry_run=false

while getopts ":hn" opt; do
    case $opt in
    h)
        drone_cli_help
        exit 0
        ;;
    n)
        dry_run=true
        ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        drone_cli_help
        exit 1
        ;;
    esac
done

shift $((OPTIND - 1))

source_tag=$1
destination_tag=$2

if [[ ! $destination_tag =~ ^(stable|latest|donotuse)$ ]]; then
    echo "destination tag needs to be stable or latest (or donotuse for testing), not ${destination_tag}"
    exit 1
fi

if [[ -z "${1}" ]]; then
    drone_cli_help
    exit 1
fi

if ! drone -v; then
    drone_cli_help
    exit 1
fi

echo "Promoting ${source_tag} to ${destination_tag}"


# get the build number for the latest Drone build with the source tag
build_number=""
page=1
until [ ${page} -gt 10 ]; do
    build_number=$(drone -s "${drone_server}" build ls rancher/rancher --page ${page} --event tag --format "{{.Number}},{{.Ref}}" | grep "${source_tag}$" | cut -d',' -f1 | head -1)

    if [ -n "${build_number}" ]; then
        break
    fi

    page=$((page + 1))

    sleep 1
done

# promote the build to promote-docker-image, 
# and to promote-stable (which publishes the Helm chart) if destination is "stable"
if [[ -n ${build_number} ]]; then
    if [[ $dry_run == true ]]; then
        echo "Dry run: drone build promote rancher/rancher ${build_number} promote-docker-image --param=SOURCE_TAG=${source_tag} --param=DESTINATION_TAG=${destination_tag}"
        if [[ $destination_tag == "stable" ]]; then
            echo "Dry run: drone build promote rancher/rancher ${build_number} promote-stable"
        fi
    else
        drone build promote rancher/rancher "${build_number}" promote-docker-image --param=SOURCE_TAG="${source_tag}" --param=DESTINATION_TAG="${destination_tag}"
        if [[ $destination_tag == "stable" ]]; then
            drone build promote rancher/rancher "${build_number}" promote-stable
        fi
    fi
    exit 0
fi

echo "No Build Found for source tag: ${source_tag}"
exit 1
