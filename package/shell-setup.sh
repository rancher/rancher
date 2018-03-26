#!/bin/bash
set -e

token=$1
cluster=$2

mkdir -p /nonexistent
mount -t tmpfs tmpfs /nonexistent
cd /nonexistent

mkdir -p .kube/certs

if [ -f /etc/kubernetes/ssl/certs/serverca ]; then
    cp /etc/kubernetes/ssl/certs/serverca .kube/certs/ca.crt
    chmod 666 .kube/certs/ca.crt
    CERT="    certificate-authority: /nonexistent/.kube/certs/ca.crt"
fi

for i in /run /var/run /etc/kubernetes; do
    mount -t tmpfs tmpfs $i
done

cat <<EOF > .kube/config
apiVersion: v1
kind: Config
clusters:
- cluster:
    api-version: v1
    server: "${CATTLE_SERVER}/k8s/clusters/${cluster}"
${CERT}
  name: "Default"
contexts:
- context:
    cluster: "Default"
    user: "Default"
  name: "Default"
current-context: "Default"
users:
- name: "Default"
  user:
    token: "${token}"
EOF

cp /etc/skel/.bashrc .
cat >> .bashrc <<EOF
PS1="> "
. /etc/bash_completion
alias k="kubectl"
alias ks="kubectl -n kube-system"
EOF

chmod 777 .kube .bashrc
chmod 666 .kube/config

for i in $(env | cut -d "=" -f 1 | grep "CATTLE\|RANCHER"); do
    unset $i
done

exec su -s /bin/bash nobody
