#!/bin/bash

set -e

if [ ! -e /run/secrets/kubernetes.io/serviceaccount ] && [ ! -e /dev/kmsg ]; then
    echo "ERROR: Rancher must be ran with the --privileged flag when running outside of Kubernetes"
    exit 1
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
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
  echo "[$(date -Iseconds)] [CgroupV2 Fix] Evacuating Root Cgroup ..."
  # move the processes from the root group to the /init group,
  # otherwise writing subtree_control fails with EBUSY.
  mkdir -p /sys/fs/cgroup/init
  xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs || :
  # enable controllers
  sed -e 's/ / +/g' -e 's/^/+/' <"/sys/fs/cgroup/cgroup.controllers" >"/sys/fs/cgroup/cgroup.subtree_control"
  echo "[$(date -Iseconds)] [CgroupV2 Fix] Done"
fi
exec tini -- rancher --http-listen-port=80 --https-listen-port=443 --audit-log-path=${AUDIT_LOG_PATH} --audit-level=${AUDIT_LEVEL} --audit-log-maxage=${AUDIT_LOG_MAXAGE} --audit-log-maxbackup=${AUDIT_LOG_MAXBACKUP} --audit-log-maxsize=${AUDIT_LOG_MAXSIZE} "${@}"
