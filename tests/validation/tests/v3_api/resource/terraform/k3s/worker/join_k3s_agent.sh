#!/bin/bash
# This script is used to join one or more nodes as agents
echo $@

mkdir -p /etc/rancher/k3s
cat <<EOF >>/etc/rancher/k3s/config.yaml
server: https://${4}:6443
token:  "${5}"
EOF

if [[ ! -z "${7}" ]] && [[ "${7}" == *":"* ]]
then
   echo -e "${7}" >> /etc/rancher/k3s/config.yaml
   cat /etc/rancher/k3s/config.yaml
fi
if [[ -n "${7}" ]] && [[ "${7}" == *"protect-kernel-defaults"* ]]
then
  cat /tmp/cis_workerconfig.yaml >> /etc/rancher/k3s/config.yaml
  echo -e "vm.panic_on_oom=0" >>/etc/sysctl.d/90-kubelet.conf
  echo -e "vm.overcommit_memory=1" >>/etc/sysctl.d/90-kubelet.conf
  echo -e "kernel.panic=10" >>/etc/sysctl.d/90-kubelet.conf
  echo -e "kernel.panic_on_oops=1" >>/etc/sysctl.d/90-kubelet.conf
  echo -e "kernel.keys.root_maxbytes=25000000" >>/etc/sysctl.d/90-kubelet.conf
  sysctl -p /etc/sysctl.d/90-kubelet.conf
  systemctl restart systemd-sysctl
fi

if [ ${1} = "rhel" ]
then
    subscription-manager register --auto-attach --username=${8} --password=${9}
    subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

export "${2}"="${3}"
if [[ "${3}" == *"v1.18"* ]] || [[ "${3}" == *"v1.17"* ]] && [[ -n "${7}" ]]
then
    echo "curl -sfL https://get.k3s.io | sh -s - ${7} --node-external-ip=${6} --server https://${4}:6443 --token ${5} ${7}"
    curl -sfL https://get.k3s.io | sh -s - ${7} --node-external-ip=${6} --server https://"${4}":6443 --token "${5}"
else
    if [ ${10} != "null" ]
    then
      curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=${10} sh -s - agent --node-external-ip=${6}
    else
      curl -sfL https://get.k3s.io | sh -s - agent --node-external-ip=${6}
    fi
fi
sleep 20
