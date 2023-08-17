#!/bin/sh
set -ue

check_go_mod() {
    local directory="$1"
    cd "$directory"
    go mod tidy
    go mod verify
    cd "$OLDPWD"
}

check_modules_diff() {
    local module_file="$1"
    local root_module_file="$2"
    local bad_module=false

    while read -r module tag; do
        roottag=$(awk -v module="$module" '$1 == module {print $2}' "$root_module_file")
        echo "${module}:"
        echo "${tag} (${root_module_file})"
        echo "${roottag} (./go.mod)"
        if [ "${tag}" != "${roottag}" ]; then
            echo "${module} is different ('${tag}' vs '${roottag}')"
            bad_module=true
        fi
    done < <(awk 'NR>1 && !/indirect/ && /rancher/ {print $1, $2}' "$module_file")

    if [ "${bad_module}" = "true" ]; then
        echo "Diff found between ${root_module_file} and ${module_file}"
        exit 1
    fi
}


for directory in . ./pkg/apis ./pkg/client; do
    check_go_mod "$directory"
done

if [ -n "$(git status --porcelain)" ]; then
    echo "go.mod is not up to date. Please 'run go mod tidy' and commit the changes."
    echo
    echo "The following go files did differ after tidying them:"
    git status --porcelain
    exit 1
fi

check_modules_diff "pkg/apis/go.mod" "go.mod"
