#!/bin/bash

LIST="rancher-images.txt"
MISSING_IMAGES=""

scanRegistry() {
    while IFS= read -r i; do
        [ -z "${i}" ] && continue
        if inspect=$(skopeo inspect --raw docker://${REGISTRY_NAME}${i} | egrep '(digest|os|arch)') > /dev/null 2>&1; then
            echo "${i} - ${inspect}"
        else
	        MISSING_IMAGES="${MISSING_IMAGES}\n${i}"
        fi
    done < "${list}"

    if [[ ${MISSING_IMAGES} ]]; then
        echo -e "\nThe following images were missing in registry ${REGISTRY_NAME}:"
        echo -e "\n${MISSING_IMAGES}" - "${inspect}"
    else
        echo -e "\nNo missing images in your registry!"
    fi
}

usage() {
	cat << EOF

$(basename "$0")

This script will scan a specified repository for Rancher images and list images that are not found.

USAGE: % ./$(basename "$0") [options]

OPTIONS:
    -h	                -> Usage
    -l | --image-list   -> Path to text file with list of images
    -r | --registry     -> Registry to use

EXAMPLES OF USAGE:

* Run script
	
    $ ./$(basename "$0") -l rancher-images.txt -r docker.io/

EOF
}

Main() {
    POSITIONAL=()
    while [[ $# -gt 0 ]]; do
        key="$1"
        case $key in
            -r|--registry)
                REGISTRY_NAME="$2"
                shift
                shift
                ;;
            -l|--image-list)
                list="$2"
                shift
                shift
                ;;
            -h|--help)
                help="true"
                shift
                ;;
            *)
                usage
                exit 1
                ;;
        esac
    done

    if [[ $help ]]; then
        usage
        exit 0
    fi

    scanRegistry
}

Main "$@"