#!/bin/bash

RANCHER_IMAGES_PATH="$(pwd)/rancher-images.txt"
RANCHER_MISSING_IMAGES_PATH="$(pwd)/missing-images.txt"
SCAN_REGISTRY_PATH="$(pwd)/scan-registry.sh"
DOCKER_IMAGES_PATH="$(pwd)/docker-hub-existing-images.txt"
USER_REGISTRY_IMAGES_PATH="$(pwd)/user-existing-images.txt"
DOCKER_REGISTRY="docker.io/"
RANCHER_VERSION="$1"
USER_REGISTRY="$2"

prereq() {
    echo -e "\nInstalling skopeo tool..."
    if skopeo -v > /dev/null 2>&1; then
        echo -e "\nSkopeo is already installed!"
    else
        . /etc/os-release

        # Ubuntu 20.10 or higher are needed for skopeo.
        [[ "${ID}" == "ubuntu" || "${ID}" == "debian" ]] && sudo apt update && sudo apt -y install skopeo
        [[ "${ID}" == "rhel" || "${ID}" == "fedora" ]] && sudo yum install skopeo -y
        [[ "${ID}" == "opensuse-leap" || "${ID}" == "sles" ]] && sudo zypper install  -y skopeo
    fi
}

scanRegistries() {
    echo -e "\nPulling rancher-images.txt file..."
    wget https://prime.ribs.rancher.io/rancher/"${RANCHER_VERSION}"/rancher-images.txt

    echo -e "\nRunning scan-registry.sh against Docker Hub..."
    "${SCAN_REGISTRY_PATH}" -l "$(pwd)/rancher-images.txt" -r "${DOCKER_REGISTRY}" >> "${DOCKER_IMAGES_PATH}"

    echo -e "\nRunning scan-registry.sh against specified registry..."
    "${SCAN_REGISTRY_PATH}" -l "$(pwd)/rancher-images.txt" -r "${USER_REGISTRY}" >> "${USER_REGISTRY_IMAGES_PATH}"
}

compareResults() {
    echo -e "\nComparing the results..."
    comm -13 <(sort -u "${DOCKER_IMAGES_PATH}" | cut -d ' ' -f1) <(sort -u "${USER_REGISTRY_IMAGES_PATH}"| cut -d ' ' -f1) >> "${RANCHER_MISSING_IMAGES_PATH}"
    sed -i '/^$/d' "${RANCHER_MISSING_IMAGES_PATH}"

    echo -e "\nImages that are missing in user specified registry:"
    if [[ ! -s "${RANCHER_MISSING_IMAGES_PATH}" ]]; then
        echo -e "\nNo missing images in your registry!"
    else
        cat "${RANCHER_MISSING_IMAGES_PATH}"
    fi
}

usage() {
	cat << EOF

$(basename "$0")

This script will run the scan-registry.sh two times; once with the Docker Hub registry and once with the specified

registry. Once done, it will compare the results and list the images that are missing in the user specified registry. 

When running the script, specify the Rancher version, prefixed with a leading 'v' and the registry to compare against 

Docker, suffixed with a trailing slash.

USAGE: % ./$(basename "$0") [options]

OPTIONS:
    -h          -> Usage

EXAMPLES OF USAGE:

* Run script
	
	$ ./$(basename "$0") v<Rancher version> <registry>/

EOF
}

while getopts "h" opt; do
	case ${opt} in
		h)
			usage
			exit 0;;
    esac
done

Main() {
    prereq
    scanRegistries
    compareResults
}

Main "$@"
