#!/bin/sh
set -ue

for DIRECTORY in . ./pkg/apis ./pkg/client; do
    cd "$DIRECTORY"
    go mod tidy
    go mod vendor
    cd "$OLDPWD"
done

if [ -n "$(git status --porcelain)" ]; then
    echo "go.mod is not up to date. Please 'run go mod tidy' and commit the changes."
    echo
    echo "The following go files did differ after tidying them:"
    git status --porcelain
    exit 1
fi
