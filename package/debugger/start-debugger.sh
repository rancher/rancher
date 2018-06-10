#!/bin/bash

#log=--log

/go/bin2/dlv exec /go/src/github.com/rancher/rancher/debug --build-flags "-tags k8s -gcflags 'all=-N -l'" -l 0.0.0.0:2345 --api-version=2 --headless $log -- --http-listen-port=80 --https-listen-port=443

