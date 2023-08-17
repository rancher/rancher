#!/bin/sh
set -ue

for DIRECTORY in . ./pkg/apis ./pkg/client; do
    cd "$DIRECTORY"
    go mod tidy
    go mod verify
    cd "$OLDPWD"
done

if [ -n "$(git status --porcelain)" ]; then
    echo "go.mod is not up to date. Please 'run go mod tidy' and commit the changes."
    echo
    echo "The following go files did differ after tidying them:"
    git status --porcelain
    exit 1
fi

# Check diff between ./go.mod and ./pkg/apis/go.mod
badmodule="false"
while read -r module tag; do
  # Get tag from module in ./go.mod
  roottag=$(sed '1,/^require/d' go.mod | grep "${module} " | awk '{ print $2 }')
  echo "${module}:"
  echo "${tag} (./pkg/apis/go.mod)"
  echo "${roottag} (./go.mod)"
  # Compare with tag from module in ./pkg/apis/go.mod
  if [ "${tag}" != "${roottag}" ]; then
    echo "${module} is different ('${tag}' vs '${roottag}')"
    badmodule="true"
  fi
done << EOF
$(sed '1,/require/d' pkg/apis/go.mod | head -n -1 | grep -v indirect | grep rancher |  awk '{ print $1,$2 }')
EOF

if [ "${badmodule}" = "true" ]; then
  echo "Diff found between ./go.mod and ./pkg/apis/go.mod"
  exit 1
fi