#!/bin/bash
set -e -x

cd $(dirname $0)/..

trash -k

rm -rf pkg/hyperkube
cp -rf vendor/k8s.io/kubernetes/cmd/hyperkube pkg/hyperkube
sed -i 's/package main/package hyperkube/g' pkg/hyperkube/*.go

trash
