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
registries:
  "docker.io":
    auth:
      username: "${DOCKERHUB_USERNAME}"
      password: "${DOCKERHUB_PASSWORD}"
EOF

if [ ! -z "${9}" ] && [[ "${9}" == *":"* ]]
then
   echo "${9}"
   echo -e "${9}" >> /etc/rancher/rke2/config.yaml
   if [[ "${9}" != *"cloud-provider-name"* ]]
   then
     echo -e "node-external-ip: ${6}" >> /etc/rancher/rke2/config.yaml
   fi
   cat /etc/rancher/rke2/config.yaml
else
  echo -e "node-external-ip: ${6}" >> /etc/rancher/rke2/config.yaml
fi

if [[ ${1} = "rhel" ]]
then
   subscription-manager register --auto-attach --username=${11} --password=${12}
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

if [ ${1} = "centos8" ] || [ ${1} = "rhel8" ]
then
  yum install tar -y
  yum install iptables -y
  workaround="[keyfile]\nunmanaged-devices=interface-name:cali*;interface-name:tunl*;interface-name:vxlan.calico;interface-name:flannel*"
  if [ ! -e /etc/NetworkManager/conf.d/canal.conf ]; then
    echo -e $workaround > /etc/NetworkManager/conf.d/canal.conf
  else
    echo -e $workaround >> /etc/NetworkManager/conf.d/canal.conf
  fi
  sudo systemctl reload NetworkManager
fi

export "${10}"="${5}"
if [ ! -z "${13}" ]
then
  export INSTALL_RKE2_METHOD="${13}"
fi

if [ ${8} = "rke2" ]
then
   if [ ${7} != "null" ]
   then
       curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=${7} INSTALL_RKE2_TYPE='agent' sh -
   else
       curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE='agent' sh -
   fi
   if [ ! -z "${9}" ] && [[ "${9}" == *"cis"* ]]
   then
       if [[ ${1} == *"rhel"* ]] || [[ ${1} == *"centos"* ]]
       then
           cp -f /usr/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
       else
           cp -f /usr/local/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
       fi
       systemctl restart systemd-sysctl
   fi
   sudo systemctl enable rke2-agent
   sudo systemctl start rke2-agent
else
   curl -sfL https://get.rancher.io | INSTALL_RANCHERD_VERSION=${5} INSTALL_RKE2_TYPE='agent' sh -
   sudo systemctl enable rancherd-agent
   sudo systemctl start rancherd-agent
fi
