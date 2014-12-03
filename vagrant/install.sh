#!/bin/bash

IMAGES="cattle/stampede-server
cattle/stampede-agent
cattle/agent-instance
cattle/libvirt
cattle/stampede-wrapper
ibuildthecloud/systemd-docker"

download_status()
{
    for i in ${IMAGES}; do
        if [ $(docker images $i | wc -l) -le 1 ]; then
            echo -e "\tPENDING DOWNLOAD: ${i}"
        fi
    done
}

while ! fleetctl list-machines >/dev/null 2>&1; do
    echo Waiting for fleet
    sleep 1
done

DOWNLOAD=/tmp/cattle-stampede.service

if [ -e $DOWNLOAD ]; then
    rm -f ${DOWNLOAD}
fi

wget -q -O $DOWNLOAD http://stampede.io/latest/cattle-stampede.service
fleetctl start $DOWNLOAD

echo "Starting Stampede in Vagrant can take 20 minutes to download everything"
sleep 5

journalctl -f | grep -v 'docker\[' &
JOURNAL=$!

while [ $(fleetctl list-units -fields active | grep -v active | wc -l) -gt 1 ]; do
    echo Waiting for units to be active: $(echo $(fleetctl list-units -fields unit,active -no-legend | grep -v active | awk '{print $1}'))
    download_status
    sleep 10
done

kill $JOURNAL

while [ "$(curl -s http://localhost:9080/ping)" != "pong" ]; do
    echo "Waiting for Stampede API"
    sleep 2
done

echo "Stampede is running on port 9080"
