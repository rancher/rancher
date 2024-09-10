#!/bin/bash
set -e

cd  /root/go/src/github.com/rancherlabs/corral-packages

make init
make build
