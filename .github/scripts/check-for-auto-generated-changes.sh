#!/bin/sh
set -ue

go generate ./...

if [ -n "$(git status --porcelain)" ]; then
    echo "Generated files have either been changed manually or were not updated."
    echo
    echo "The following generated files did differ after regeneration:"
    git status --porcelain
    echo
    echo "Changes made by regeneration:"
    git --no-pager diff --stat
    git --no-pager diff
    exit 1
fi
