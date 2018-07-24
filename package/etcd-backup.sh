#!/bin/bash
set -e

BACKUP_INTERVAL=${ETCD_BACKUP_INTERVAL:-60}
RETENTION_INTERVAL=${ETCD_BACKUP_RETENTION:-1440}
BACKUP_DIR=${ETCD_BACKUP_DIR:-/var/lib/rancher/etcd-backup}

export ETCDCTL_API=3
export ETCDCTL_CACERT=/etc/kubernetes/ssl/kube-ca.pem
export ETCDCTL_CERT=/etc/kubernetes/ssl/kube-etcd-127-0-0-1.pem
export ETCDCTL_KEY=/etc/kubernetes/ssl/kube-etcd-127-0-0-1-key.pem
export ETCDCTL_ENDPOINTS=https://localhost:2379

log() {
  echo $(date "+%F %T") "etcd_backup" "$@"
}

mkdir -p ${BACKUP_DIR}
# simple backup lookup
while true
do
  sleep ${BACKUP_INTERVAL}m
        log "Starting etcd backup.."
  # is etcd alive ?
  etcdctl member list > /dev/null 2>&1 || continue
  etcdctl snapshot save ${BACKUP_DIR}/etcd-backup_$(date "+%F_%H-%M").db || err=$?
  if [ -n "$err" ]
  then
    log "Etcd bakcup failed.."
    continue
  fi

# clean up, we skip this incase of a failed backup because we don't want to remove existing backups if there is a problem
log "Running etcd backup cleanup.."
err=""
find ${BACKUP_DIR} -type f -mmin +${RETENTION_INTERVAL} -print -delete || err=$?
if [ -n "$err" ]
  then
    log "Etcd backup cleanup failed.."
    continue
  fi

done
