#!/bin/bash
# This script is used to join one or more nodes as agents
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

echo "$7"
if [[ ! -z "$7" ]] && [[ "$7" == *":"* ]]
then
   echo "$7"
   echo -e "$7" >> /etc/rancher/rke2/config.yaml
   cat /etc/rancher/rke2/config.yaml
fi

if [ ${1} = "rhel" ]
then
   subscription-manager register --auto-attach --username=${8} --password=${9}
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

if [[ ${1} = "centos8" ]] || [[ ${1} = "rhel8" ]]
then
  yum install tar -y
  yum install iptables -y
fi

if [ ${6} = "rke2" ]
then
   curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=${5} INSTALL_RKE2_TYPE='agent' sh -
   sudo systemctl enable rke2-agent
   sudo systemctl start rke2-agent
else
   curl -sfL https://get.rancher.io | INSTALL_RANCHERD_VERSION=${5} INSTALL_RKE2_TYPE='agent' sh -
   sudo systemctl enable rancherd-agent
   sudo systemctl start rancherd-agent
fi
