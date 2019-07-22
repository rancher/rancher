#!/bin/bash
set -x

sudo apt-get update
if sudo apt-get install -y nfs-kernel-server ; then
  echo "install nfs-kernel-server successfully, continue "
else
  echo "fixing the error"
  sudo rm /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock*
  sudo dpkg --configure -a
  sudo apt-get install -y nfs-kernel-server
fi

sudo mkdir -p /nfs
sudo chown nobody:nogroup /nfs
echo  "/nfs    *(rw,sync,no_subtree_check)" | sudo tee  /etc/exports
sudo exportfs -a
sudo service nfs-kernel-server start
sudo ufw allow 2049
