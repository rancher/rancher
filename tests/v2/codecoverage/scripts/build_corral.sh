#!/bin/bash
set -e

cd  /go/src/github.com/rancherlabs/corral

echo "building corral bin"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build 

mv corral /go/bin