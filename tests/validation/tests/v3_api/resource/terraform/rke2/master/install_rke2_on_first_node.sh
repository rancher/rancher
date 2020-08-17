#!/bin/bash
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

if [ ${1} = "rhel" ]
then
    subscription-manager register --auto-attach --username=${4} --password=${5}
    subscription-manager repos --enable=rhel-7-server-extras-rpms
fi


curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=${3} sh -
sudo systemctl enable rke2-server
sudo systemctl start rke2-server

export KUBECONFIG=/etc/rancher/rke2/rke2.yaml PATH=$PATH:/var/lib/rancher/rke2/bin

IFS=$'\n'
notready=false
timeElapsed=0
sleep 60

while [[ $timeElapsed -lt 240 ]]
do
   for rec in `kubectl get nodes --kubeconfig=/etc/rancher/rke2/rke2.yaml`
   do
       echo $rec
       if [[ "$rec" == *"NotReady"* ]]
       then
          notready=true
       fi
   done
   echo "Node state $notready"
   if [[ $notready == false ]]
   then
        break
   fi
   sleep 20
   timeElapsed=`expr $timeElapsed + 20`
done

cat /etc/rancher/rke2/config.yaml> /tmp/joinflags
cat /var/lib/rancher/rke2/server/node-token >/tmp/nodetoken
cat /etc/rancher/rke2/rke2.yaml >/tmp/config