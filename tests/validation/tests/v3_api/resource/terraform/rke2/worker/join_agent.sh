#!/bin/bash
set -x
echo $@
hostname=`hostname -f`
mkdir -p /etc/rancher/rke2
cat <<EOF >>/etc/rancher/rke2/config.yaml
server: https://${3}:9345
token:  "${4}"
node-name: "${hostname}"
cloud-provider-name:  "aws"
EOF

if [ ${1} = "rhel" ]
then
    subscription-manager register --auto-attach --username=${6} --password=${7}
    subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=${5} INSTALL_RKE2_TYPE='agent' sh -
systemctl enable rke2-agent
systemctl start rke2-agent