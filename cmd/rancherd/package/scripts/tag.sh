#!/bin/sh -xe

REMOTE=${REMOTE:=rancher}

info()
{
    echo '[INFO] ' "$@"
}

fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

clean_tag() {
  set +e
  if [ -z "$TAG" ]; then
    fatal "TAG can not be empty"
  fi
  git push -d ${REMOTE} ${TAG}
  git tag -d ${TAG}
  set -e
}

tag_branch() {
  if [ -z "$TAG" ]; then
    fatal "TAG can not be empty"
  fi
  git fetch ${REMOTE}
  git checkout ${REMOTE}/master
  git tag ${TAG}
}

push_tag() {
  if [ -z "$TAG" ]; then
    fatal "TAG can not be empty"
  fi
  git push ${REMOTE} tag ${TAG}
}


TAGS=$1
if [ -z "$TAGS" ]; then
  fatal "You need to pass tags as argument"
fi
for tag in $TAGS; do
  TAG=$tag clean_tag
  TAG=$tag tag_branch
  TAG=$tag push_tag
done