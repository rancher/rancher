#!/bin/bash

set -e

if [ ! -e /run/secrets/kubernetes.io/serviceaccount ] ; then
    echo "ERROR: Rancher expects serviceaccount credentials to be mounted in the filesystem"
    exit 1
fi

git_dirs=$(find /var/lib/rancher-data/local-catalogs -type d -name '.git')
echo "Restoring git repositories: "
for dir in ${git_dirs[@]}; do
  echo "- ${dir}"
  cd "${dir}/.." && git checkout HEAD && cd -
done

update-ca-certificates

exec catatonit -- rancher --http-listen-port=80 --https-listen-port=443 --audit-log-path=${AUDIT_LOG_PATH} --audit-level=${AUDIT_LEVEL} --audit-log-maxage=${AUDIT_LOG_MAXAGE} --audit-log-maxbackup=${AUDIT_LOG_MAXBACKUP} --audit-log-maxsize=${AUDIT_LOG_MAXSIZE} "${@}"
