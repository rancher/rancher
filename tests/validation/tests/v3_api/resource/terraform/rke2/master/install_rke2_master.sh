#!/bin/bash
# This script installs the first master, ensuring first master is installed
# and ready before proceeding to install other nodes
set -x
echo $@
hostname=`hostname -f`
mkdir -p /etc/rancher/rke2
cat << EOF >/etc/rancher/rke2/config.yaml
write-kubeconfig-mode: "0644"
tls-san:
  - ${2}
node-name: ${hostname}
EOF

if [ ! -z "${7}" ] && [[ "${7}" == *":"* ]]
then
   echo "${7}"
   echo -e "${7}" >> /etc/rancher/rke2/config.yaml
   if [[ "${7}" != *"cloud-provider-name"* ]]
   then
     echo -e "node-external-ip: ${4}" >> /etc/rancher/rke2/config.yaml
   fi
   cat /etc/rancher/rke2/config.yaml
else
  echo -e "node-external-ip: ${4}" >> /etc/rancher/rke2/config.yaml
fi

if [[ ${1} = "rhel" ]]
then
   subscription-manager register --auto-attach --username=${9} --password=${10}
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

export "${8}"="${3}"
if [ ! -z "${11}" ]
then
  export INSTALL_RKE2_METHOD="${11}"
fi

if [ ${6} = "rke2" ]
then
   if [ ${5} != "null" ]
   then
       curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=${5} sh -
   else
       curl -sfL https://get.rke2.io | sh -
   fi
   sleep 10
   if [ ! -z "${7}" ] && [[ "${7}" == *"cis"* ]]
   then
       if [[ ${1} == *"rhel"* ]] || [[ ${1} == *"centos"* ]]
       then
           cp -f /usr/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
       else
           cp -f /usr/local/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
       fi
       systemctl restart systemd-sysctl
       useradd -r -c "etcd user" -s /sbin/nologin -M etcd -U
   fi
   sudo systemctl enable rke2-server
   sudo systemctl start rke2-server
else
   curl -sfL https://get.rancher.io | INSTALL_RANCHERD_VERSION=${3} sh -
   sudo systemctl enable rancherd-server
   sudo systemctl start rancherd-server
fi

timeElapsed=0
while [[ $timeElapsed -lt 600 ]]
do
  notready=false
  if [[ ! -f /var/lib/rancher/rke2/server/node-token ]] || [[ ! -f /etc/rancher/rke2/rke2.yaml ]]
  then
    notready=true
  fi
  if [[ $notready == false ]]
  then
    break
  fi
  sleep 5
  timeElapsed=`expr $timeElapsed + 5`
done

cat /etc/rancher/rke2/config.yaml> /tmp/joinflags
cat /var/lib/rancher/rke2/server/node-token >/tmp/nodetoken
cat /etc/rancher/rke2/rke2.yaml >/tmp/config
