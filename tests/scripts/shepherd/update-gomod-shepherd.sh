#!/bin/bash
if [ "$1" == "-h" ] || [ -z "$1" ]; then
  echo "Usage: `basename $0` <path-to-upstream-shepherd> <shepherd-release-branch> <path-to-rancher>" 
  echo "WARN: this will update your local go.mod and go.sum files in your rancher repo."
  echo -e "Requirements:\n* local clone of upstream rancher/shepherd\n* clone of rancher fork with local changes"
  echo "example: `basename $0` ~/upstream-shepherd release/v2.8 ~/rancher-fork"
  exit 0
fi

if [ ! -d $1 ]; then
  echo "$1 not a valid path"
  exit 1
fi

if [ ! -d $3 ]; then
  echo "$3 is not a valid path"
  exit 1
fi

echo "getting latest shepherd version for branch $2"
cd $1
git checkout $2 -q
git fetch -q
git pull -q
export SHEPHERD_VERSION=$(curl -s https://proxy.golang.org/github.com/rancher/shepherd/@v/$(git log -n 1 --pretty=format:"%H").info | grep -E -o "\bv0.0.0-+[A-Za-z0-9.-]+[A-Za-z0-9.-]\b")
echo "Shepherd Version is: $SHEPHERD_VERSION"

echo "writing version to go.mod, then tidying in $3"
cd $3

if [ $upstream == "rancher" ]; then
  go get github.com/rancher/shepherd@$SHEPHERD_VERSION
else
  go mod edit -replace=github.com/rancher/shepherd=github.com/$upstream/shepherd@$SHEPHERD_VERSION
fi

go mod tidy
