#!/bin/bash
set -e

NAME=$1

if [[ -z "$1" ]]
  then
    echo "No name supplied"
    exit 1
fi

# Build the jail directory structure
mkdir -p /opt/jail/$NAME/dev
mkdir -p /opt/jail/$NAME/etc/ssl/certs
mkdir -p /opt/jail/$NAME/usr/bin
mkdir -p /opt/jail/$NAME/management-state/node/nodes
mkdir -p /opt/jail/$NAME/var/lib/rancher/management-state/bin
mkdir -p /opt/jail/$NAME/management-state/bin
mkdir -p /opt/jail/$NAME/tmp

# Copy over required files to the jail
if [[ -d /lib64 ]]; then
  cp -r /lib64 /opt/jail/$NAME
fi

cp -r /lib /opt/jail/$NAME
cp -r /usr/lib /opt/jail/$NAME/usr
cp /etc/ssl/certs/ca-certificates.crt /opt/jail/$NAME/etc/ssl/certs
cp /etc/resolv.conf /opt/jail/$NAME/etc/
cp /etc/passwd /opt/jail/$NAME/etc/
cp /etc/hosts /opt/jail/$NAME/etc/
cp /etc/nsswitch.conf /opt/jail/$NAME/etc/

if [ -d /var/lib/rancher/management-state/bin ] && [ "$(ls -A /var/lib/rancher/management-state/bin)" ]; then
  ( cd /var/lib/rancher/management-state/bin
    for f in *; do
      if [ ! -f "/opt/drivers/management-state/bin/$f" ]; then
        cp "$f" "/opt/drivers/management-state/bin/$f"
      fi
    done
  )
fi

if [[ -f /etc/ssl/certs/ca-additional.pem ]]; then
  cp /etc/ssl/certs/ca-additional.pem /opt/jail/$NAME/etc/ssl/certs
fi

if [[ -f /etc/rancher/ssl/cacerts.pem ]]; then
  cp /etc/rancher/ssl/cacerts.pem /opt/jail/$NAME/etc/ssl/certs
fi

# Hard link driver binaries
cp -r -l /opt/drivers/management-state/bin /opt/jail/$NAME/var/lib/rancher/management-state

# Hard link rancher-machine into the jail
cp -l /usr/bin/rancher-machine /opt/jail/$NAME/usr/bin

# Hard link helm into the jail
cp -l /usr/bin/rancher-helm /opt/jail/$NAME/usr/bin

# Hard link tiller into the jail
cp -l /usr/bin/rancher-tiller /opt/jail/$NAME/usr/bin

# Hard link ssh into the jail
cp -l /usr/bin/ssh /opt/jail/$NAME/usr/bin

cd /dev
# tar copy /dev excluding mqueue and shm
tar cf - --exclude=mqueue --exclude=shm --exclude=pts . | (cd /opt/jail/${NAME}/dev; tar xfp -)

touch /opt/jail/$NAME/done
