#!/bin/bash
set -x

trap 'exit 0' SIGTERM
bash -c "$1"
