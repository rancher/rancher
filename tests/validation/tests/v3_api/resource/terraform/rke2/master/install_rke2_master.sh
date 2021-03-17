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
cloud-provider-name:  "aws"
node-name: ${hostname}
EOF

if [ ! -z "${6}" ] && [[ "${6}" == *":"* ]]
then
   echo "${6}"
   echo -e "${6}" >> /etc/rancher/rke2/config.yaml
   cat /etc/rancher/rke2/config.yaml
fi

if [[ ${1} == *"rhel"* ]]
then
   subscription-manager register --auto-attach --username=${7} --password=${8}
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

if [ ${1} = "centos8" ] || [ ${1} = "rhel8" ]
then
  yum install tar -y
  yum install iptables -y
fi

if [ ${5} = "rke2" ]
then
   if [ ${4} != "null" ]
   then
       curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=${3}  INSTALL_RKE2_CHANNEL=${4} sh -
   else
       curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=${3} sh -
   fi
   sleep 10
   if [ ! -z "${6}" ] && [[ "${6}" == *"cis"* ]]
   then
       if [[ ${1} == *"rhel"* ]] || [[ ${1} == *"centos"* ]]
       then
           cp -f /usr/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
       else
           cp -f /usr/local/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
       fi
       systemctl restart systemd-sysctl
       useradd -r -c "etcd user" -s /sbin/nologin -M etcd
   fi
   sudo systemctl enable rke2-server
   sudo systemctl start rke2-server
else
   curl -sfL https://get.rancher.io | INSTALL_RANCHERD_VERSION=${3} sh -
   sudo systemctl enable rancherd-server
   sudo systemctl start rancherd-server
fi

export KUBECONFIG=/etc/rancher/rke2/rke2.yaml PATH=$PATH:/var/lib/rancher/rke2/bin

timeElapsed=0
while ! `kubectl get nodes >/dev/null 2>&1` && [[ $timeElapsed -lt 300 ]]
do
   sleep 5
   timeElapsed=`expr $timeElapsed + 5`
done

IFS=$'\n'
timeElapsed=0
while [[ $timeElapsed -lt 540 ]]
do
   notready=false
   for rec in `kubectl get nodes`
   do
      if [[ "$rec" == *"NotReady"* ]]
      then
         notready=true
      fi
  done
  if [[ $notready == false ]]
  then
     break
  fi
  sleep 20
  timeElapsed=`expr $timeElapsed + 20`
done

IFS=$'\n'
timeElapsed=0
while [[ $timeElapsed -lt 540 ]]
do
   helmPodsNR=false
   systemPodsNR=false
   for rec in `kubectl get pods -A --no-headers`
   do
      if [[ "$rec" == *"helm-install"* ]] && [[ "$rec" != *"Completed"* ]]
      then
         helmPodsNR=true
      elif [[ "$rec" != *"helm-install"* ]] && [[ "$rec" != *"Running"* ]]
      then
         systemPodsNR=true
      else
         echo ""
      fi
   done

   if [[ $systemPodsNR == false ]] && [[ $helmPodsNR == false ]]
   then
      break
   fi
   sleep 20
   timeElapsed=`expr $timeElapsed + 20`
done

cat /etc/rancher/rke2/config.yaml> /tmp/joinflags
cat /var/lib/rancher/rke2/server/node-token >/tmp/nodetoken
cat /etc/rancher/rke2/rke2.yaml >/tmp/config
