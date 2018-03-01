#!/bin/bash
set -e

error()
{
    echo "ERROR:" "$@" 1>&2
}

AGENT_IMAGE=${AGENT_IMAGE:-ubuntu:14.04}

export CATTLE_ADDRESS
export CATTLE_INTERNAL_ADDRESS
export CATTLE_NODE_NAME
export CATTLE_ROLE
export CATTLE_SERVER
export CATTLE_TOKEN

while true; do
    case "$1" in
        -d | --debug)              DEBUG=true                 ;;
        -s | --server)      shift; CATTLE_SERVER=$1           ;;
        -t | --token)       shift; CATTLE_TOKEN=$1            ;;
        -c | --ca-checksum) shift; CATTLE_CA_CHECKSUM=$1      ;;
        -a | --all-roles)          ALL=true                   ;;
        -e | --etcd)               ETCD=true                  ;;
        -w | --worker)             WORKER=true                ;;
        -p | --controlplane)       CONTROL=true               ;;
        -n | --node-name)          CATTLE_NODE_NAME=true      ;;
        --address)          shift; CATTLE_ADDRESS=$1          ;;
        --internal-address) shift; CATTLE_INTERNAL_ADDRESS=$1 ;;
        *) break;
    esac
    shift
done

if [ "$DEBUG" = true ]; then
    set -x
fi

if [ "$CATTLE_CLUSTER" != "true" ]; then
    if [ ! -w /var/run/docker.sock ] || [ ! -S /var/run/docker.sock ]; then
        error Please bind mount in the docker socket to /var/run/docker.sock
        error example:  docker run -v /var/run/docker.sock:/var/run/docker.sock ...
        exit 1
    fi
fi

if [ -z "$CATTLE_NODE_NAME" ]; then
    CATTLE_NODE_NAME=$(hostname -s)
fi

if [ "$CATTLE_ADDRESS" = "awslocal" ]; then
    export CATTLE_ADDRESS=$(curl -s http://169.254.169.254/latest/meta-data/local-ipv4)
elif [ "$CATTLE_ADDRESS" = "ipify" ]; then
    export CATTLE_ADDRESS=$(curl -s https://api.ipify.org)
fi

if [ -z "$CATTLE_ADDRESS" ]; then
    CATTLE_ADDRESS=$(ip route get 8.8.8.8 | grep via | awk '{print $NF}')
fi

if [ "$ALL" = true ]; then
    CATTLE_ROLE="etcd,worker,controlplane"
else
    if [ "$ETCD" = true ]; then
        CATTLE_ROLE="${CATTLE_ROLE},etcd"
    fi
    if [ "$WORKER" = true ]; then
        CATTLE_ROLE="${CATTLE_ROLE},worker"
    fi
    if [ "$CONTROL" = true ]; then
        CATTLE_ROLE="${CATTLE_ROLE},controlplane"
    fi
fi

if [ -n "$CATTLE_CA_CHECKSUM" ]; then
    temp=$(mktemp)
    curl --insecure -s -fL $CATTLE_SERVER/v3/settings/cacerts | jq -r .value > $temp
    cat $temp
    if [ "$(sha256sum $temp | awk '{print $1}')" != $CATTLE_CA_CHECKSUM ]; then
        rm -f $temp
        error $CATTLE_SERVER/v3/settings/cacerts does not match $CATTLE_CA_CHECKSUM
        exit 1
    fi
    mkdir -p /etc/kubernetes/ssl/certs
    mv $temp /etc/kubernetes/ssl/certs/serverca
fi

if [ -z "$CATTLE_SERVER" ]; then
    error -- --server is a required option
    exit 1
fi

if [ "$CATTLE_CLUSTER" != "true" ]; then
    if [ -z "$CATTLE_TOKEN" ]; then
        error -- --token is a required option
        exit 1
    fi

    if [ -z "$CATTLE_ADDRESS" ]; then
        error -- --address is a required option
        exit 1
    fi
fi

exec agent
