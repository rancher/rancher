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
mkdir -p /opt/jail/$NAME/etc/ssl
mkdir -p /opt/jail/$NAME/usr/bin
mkdir -p /opt/jail/$NAME/management-state/node/nodes
mkdir -p /opt/jail/$NAME/var/lib/rancher/management-state/bin
mkdir -p /opt/jail/$NAME/management-state/bin
mkdir -p /opt/jail/$NAME/tmp
mkdir -p /opt/jail/$NAME/bin

# Copy over required files to the jail
if [[ -d /lib64 ]]; then
  cp -r /lib64 /opt/jail/$NAME
  cp -r /usr/lib64 /opt/jail/$NAME/usr
fi

cp -r /lib /opt/jail/$NAME
cp -r /usr/lib /opt/jail/$NAME/usr
cp /var/lib/ca-certificates/ca-bundle.pem /opt/jail/$NAME/etc/ssl
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
  cp /etc/ssl/certs/ca-additional.pem /opt/jail/$NAME/etc/ssl
fi

if [[ -f /etc/rancher/ssl/cacerts.pem ]]; then
  cp /etc/rancher/ssl/cacerts.pem /opt/jail/$NAME/etc/ssl
fi

# Hard link driver binaries
cp -r -l /opt/drivers/management-state/bin /opt/jail/$NAME/var/lib/rancher/management-state

# Hard link rancher-machine into the jail
cp -l /usr/bin/rancher-machine /opt/jail/$NAME/usr/bin

# Hard link helm_2 into the jail
cp -l /usr/bin/rancher-helm /opt/jail/$NAME/usr/bin

# Hard link helm_3 into the jail
cp -l /usr/bin/helm_v3 /opt/jail/$NAME/usr/bin

# Hard link tiller into the jail
cp -l /usr/bin/rancher-tiller /opt/jail/$NAME/usr/bin

# Hard link kustomize into the jail
cp -l /usr/bin/kustomize /opt/jail/$NAME/usr/bin

# Hard link custom kustomize script into jail
cp -l /usr/bin/kustomize.sh /opt/jail/$NAME

# Hard link ssh into the jail
cp -l /usr/bin/ssh /opt/jail/$NAME/usr/bin

# Hard link nc into the jail
cp -l /usr/bin/nc /opt/jail/$NAME/usr/bin

# Hard link cat into the jail
cp -l /bin/cat /opt/jail/$NAME/bin/

# Hard link bash into the jail
cp -l /bin/bash /opt/jail/$NAME/bin/

# Hard link sh into the jail
cp -l /bin/sh /opt/jail/$NAME/bin/

# Hard link rm into the jail
cp -l /bin/rm /opt/jail/$NAME/bin/

# Hard link mkisofs into the jail
cp -l /usr/bin/mkisofs /opt/jail/$NAME/usr/bin

cd /dev
# tar copy a minimum set of devices from /dev
tar cf - zero urandom tty stdout stdin stderr random null fd core full | (cd /opt/jail/${NAME}/dev; tar xfp -)

touch /opt/jail/$NAME/done
