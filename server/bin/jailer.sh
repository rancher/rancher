#!/bin/bash
set -e

BASE=$1

if [[ -z "$1" ]]
  then
    echo "No base path supplied"
    exit 1
fi

# Build the jail directory structure
mkdir -p $BASE/dev
mkdir -p $BASE/etc/ssl/certs
mkdir -p $BASE/usr/bin
mkdir -p $BASE/usr/local/bin

# Copy over required files to the jail
cp -r /lib $BASE
cp -r /lib64 $BASE
cp /etc/ssl/certs/ca-certificates.crt $BASE/etc/ssl/certs
cp /etc/resolv.conf $BASE/etc/

# Copy driver binaries
cp -a /usr/local/bin/. $BASE/usr/local/bin/

# Copy docker-machine into the jail 
if [[ ! -x "$BASE/usr/bin/docker-machine" ]]; then
    cp /usr/bin/docker-machine $BASE/usr/bin
fi

cd /dev
# tar copy /dev excluding mqueue and shm
tar cf - --exclude=mqueue --exclude=shm . | (cd ${BASE}/dev; tar xfp -)
