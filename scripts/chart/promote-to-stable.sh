#!/bin/bash

if [[ -z "${1}" ]]; then
    echo "Promote a chart in latest to stable."
    echo "  $0 <tag>"
    echo ""
    echo "  Requires a GitHub Personal Access Token exported as GH_TOKEN"
    echo "    Create a token at https://github.com/settings/tokens"
    echo "    Permissions: repo_deployment"
    exit 1
fi

if [[ -z "${GH_TOKEN}" ]]; then
    echo 'ERROR: Missing $GH_TOKEN'
    echo 'Create a GitHub Personal Access Token and export it as GH_TOKEN'
    echo 'Required permissions: repo_deployment'
    echo 'https://github.com/settings/tokens'
    exit 1
fi

data=$(cat <<EOF
{
    "ref": "refs/tags/${1}",
    "environment": "promote-stable",
    "auto_merge": false,
    "description": "promote-to-stable.sh"
}
EOF
)

echo $data | curl -H "Authorization: token ${GH_TOKEN}" -H "Content-Type: application/json" \
-X POST https://api.github.com/repos/rancher/rancher/deployments -d @-
