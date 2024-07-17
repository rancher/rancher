#!/bin/bash

set -e

if [ ! -e /run/secrets/kubernetes.io/serviceaccount ] && [ ! -e /dev/kmsg ]; then
    echo "ERROR: Rancher must be ran with the --privileged flag when running outside of Kubernetes"
    exit 1
fi

git_dirs=(
  /var/lib/rancher-data/local-catalogs/system-library
  /var/lib/rancher-data/local-catalogs/helm3-library
  /var/lib/rancher-data/local-catalogs/library
  /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9/
  /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974/
  /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721/
)
echo "Restoring git repositories: "
for dir in ${git_dirs[@]}; do
  echo "- ${dir}"
  cd "${dir}" && git restore .
done

#########################################################################################################################################
# DISCLAIMER                                                                                                                            #
# Copied from https://github.com/moby/moby/blob/ed89041433a031cafc0a0f19cfe573c31688d377/hack/dind#L28-L37                              #
# Permission granted by Akihiro Suda <akihiro.suda.cz@hco.ntt.co.jp> (https://github.com/rancher/k3d/issues/493#issuecomment-827405962) #
# Moby License Apache 2.0: https://github.com/moby/moby/blob/ed89041433a031cafc0a0f19cfe573c31688d377/LICENSE                           #
#########################################################################################################################################
# only run this if rancher is not running in kubernetes cluster
if [ ! -e /run/secrets/kubernetes.io/serviceaccount ] && [ -f /sys/fs/cgroup/cgroup.controllers ]; then
  # move the processes from the root group to the /init group,
  # otherwise writing subtree_control fails with EBUSY.
  mkdir -p /sys/fs/cgroup/init
  xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs || :
  # enable controllers
  sed -e 's/ / +/g' -e 's/^/+/' <"/sys/fs/cgroup/cgroup.controllers" >"/sys/fs/cgroup/cgroup.subtree_control"
fi

rm -f /var/lib/rancher/k3s/server/cred/node-passwd
if [ -e /var/lib/rancher/management-state/etcd ] && [ ! -e /var/lib/rancher/k3s/server/db/etcd ]; then
  mkdir -p /var/lib/rancher/k3s/server/db
  ln -sf /var/lib/rancher/management-state/etcd /var/lib/rancher/k3s/server/db/etcd
  echo -n 'default' > /var/lib/rancher/k3s/server/db/etcd/name
fi
if [ -e /var/lib/rancher/k3s/server/db/etcd ]; then
  echo "INFO: Running k3s server --cluster-init --cluster-reset"
  set +e
  k3s server --cluster-init --cluster-reset &> ./k3s-cluster-reset.log
  K3S_CR_CODE=$?
  if [ "${K3S_CR_CODE}" -ne 0 ]; then
    echo "ERROR:" && cat ./k3s-cluster-reset.log
    rm -f /var/lib/rancher/k3s/server/db/reset-flag
    exit ${K3S_CR_CODE}
  fi
  set -e
fi
if [ -x "$(command -v update-ca-certificates)" ]; then
  update-ca-certificates
fi
if [ -x "$(command -v c_rehash)" ]; then
  c_rehash
fi
exec tini -- rancher --http-listen-port=80 --https-listen-port=443 --audit-log-path=${AUDIT_LOG_PATH} --audit-level=${AUDIT_LEVEL} --audit-log-maxage=${AUDIT_LOG_MAXAGE} --audit-log-maxbackup=${AUDIT_LOG_MAXBACKUP} --audit-log-maxsize=${AUDIT_LOG_MAXSIZE} "${@}"
