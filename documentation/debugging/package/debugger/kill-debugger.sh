#!/bin/bash
debugger=rancher/rancher/debug
pids=$(pgrep -f $debugger)

if [[ ! -z $pids ]]; then
   pgrep -f $debugger | xargs kill
fi

