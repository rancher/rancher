#!/bin/bash
# This script will create a txt file which will be used to print components versions on tag
set -e -x

echo "Creating ./bin/rancher-components.txt"

cd $(dirname $0)/..

mkdir -p bin

COMPONENTSFILE=./bin/rancher-components.txt

echo "# Components" > $COMPONENTSFILE

printf '%s\n' "$(grep "_VERSION" ./package/Dockerfile | grep ENV | egrep -v "http|\\$" | grep CATTLE |sed 's/CATTLE_//g' | sed 's/=/ /g' | grep UI | awk '{ print $2,$3 }')" >> $COMPONENTSFILE

printf '%s\n' "$(grep "rancher/" ./go.mod | egrep -v "\./"  | egrep "rke|machine" | sort -r |  awk -F'/' '{ print $NF }' | awk '$1 = toupper($1)')" >> $COMPONENTSFILE

printf '%s\n' "$(grep "_VERSION" ./package/Dockerfile | grep ENV | egrep -v "http|\\$" | grep CATTLE |sed 's/CATTLE_//g' | sed 's/=/ /g' | grep -v UI | awk '{ print $2,$3 }' | sort)" >> $COMPONENTSFILE

printf '%s\n' "$(grep "rancher/" ./go.mod | egrep -v "\./"  | egrep -v "rke|machine|\/pkg\/apis|\/pkg\/client|^module" | grep -v "=>" | awk -F'/' '{ print $NF }' | awk '$1 = toupper($1)' | sort)" >> $COMPONENTSFILE


echo "Done creating ./bin/rancher-components.txt"
