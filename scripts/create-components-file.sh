#!/bin/bash
# This script will create a txt file with -rc images/components which will be used as (pre) release description by Drone
set -e -x

echo "Creating ./bin/rancher-components.txt"

cd "$(dirname "$0")/.."

mkdir -p bin

COMPONENTSFILE=./bin/rancher-components.txt

FILES=(
    "./Dockerfile.dapper"
    "./go.mod"
    "./package/Dockerfile"
    "./pkg/apis/go.mod"
    "./pkg/client/go.mod"
    "./pkg/settings/setting.go"
    "./scripts/package-env"
)

IMAGE_FILES=(
    "./bin/rancher-images.txt"
    "./bin/rancher-windows-images.txt"
)

generate_section() {
    local pattern="$1"
    local label="$2"
    local type="$3"
    {
        echo " "
        echo "# $label"
    } >> "$COMPONENTSFILE"

    for file in "${FILES[@]}"; do
        if [[ "$file" == "./scripts/package-env" ]]; then
            grep -h -n -E "$pattern" "$file" | grep -i -E "$type" | sed -E "s|^([^:]*):(.*)|\* \2 (file $file, line \1)|"
        else
            grep -h -n -E "$pattern" "$file" | grep -i -E "$type" | grep -v "// indirect" | awk -F':' -v file="$file" -v label="$label" '{ sub(/^[ \t]+/, "", $2); sub(/[ \t]+/, " ", $2); print "* " $2 " (file " file ", line " $1 ")" }'
        fi
    done | sort -u >> "$COMPONENTSFILE"
}

echo "# Images with -rc" > "$COMPONENTSFILE"
for file in "${IMAGE_FILES[@]}"; do
    grep -h -n -E "rc[.]?[0-9]+" "$file" | awk -F':' -v file="$file" '{ sub(/^[ \t]+/, "", $2); sub(/[ \t]+/, " ", $2); print "* " $2 " (file " file ", line " $1 ")" }'
done | sort -u >> "$COMPONENTSFILE"

generate_section "rc[.]?[0-9]+" "Components with -rc"

{ 
    echo ""
    echo "# Min version components with -rc" 
} >> $COMPONENTSFILE
printf '%s\n' "$(grep -n -E "_MIN_VERSION" ./package/Dockerfile | grep ENV | grep CATTLE |sed 's/CATTLE_//g' | sed 's/=/ /g' |  awk -F':' '{ sub(/^[ \t]+/, "", $2); print "* " $2 " (file /package/Dockerfile, line " $1 ")" }' | sort | grep "\-rc")" >> $COMPONENTSFILE

K8SVERSIONSFILE=./bin/rancher-rke-k8s-versions.txt

if [[ -f "$K8SVERSIONSFILE" ]]; then
    { 
        echo ""
        echo "# RKE Kubernetes versions"
        cat $K8SVERSIONSFILE 
    } >> $COMPONENTSFILE
fi

generate_section "dev-v2.[0-9]+" "KDM References with dev branch" "kdm"
generate_section "dev-v2.[0-9]+" "Chart References with dev branch" "chart"

echo "Done creating ./bin/rancher-components.txt"
