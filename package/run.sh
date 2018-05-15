#!/bin/bash
set -e

if [ "$1" = "--" ]; then
    shift 1
    exec "$@"
fi

error()
{
    echo "ERROR:" "$@" 1>&2
}

warn()
{
    echo "WARN :" "$@" 1>&2
}

get_address()
{
    local address=$1
    # If nothing is given, return empty (it will be automatically determined later if empty)
    if [ -z $address ]; then
        echo ""
    # If given address is a network interface on the system, retrieve configured IP on that interface (only the first configured IP is taken)
    elif [ -n "$(find /sys/devices -name $address)" ]; then
        echo $(ip addr show dev $address | grep -w inet | awk '{print $2}' | cut -f1 -d/ | head -1)
    # Loop through cloud provider options to get IP from metadata, if not found return given value
    else
        case $address in
            awslocal)
                echo $(curl -s http://169.254.169.254/latest/meta-data/local-ipv4)
                ;;
            awspublic)
                echo $(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
                ;;
            doprivate)
                echo $(curl -s http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address)
                ;;
            dopublic)
                echo $(curl -s http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address)
                ;;
            azprivate)
                echo $(curl -s -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-08-01&format=text")
                ;;
            azpublic)
                echo $(curl -s -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text")
                ;;
            gceinternal)
                echo $(curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip)
                ;;
            gceexternal)
                echo $(curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip)
                ;;
            packetlocal)
                echo $(curl -s https://metadata.packet.net/2009-04-04/meta-data/local-ipv4)
                ;;
            packetpublic)
                echo $(curl -s https://metadata.packet.net/2009-04-04/meta-data/public-ipv4)
                ;;
            ipify)
                echo $(curl -s https://api.ipify.org)
                ;;
            *)
                echo $address
                ;;
        esac
    fi
}

AGENT_IMAGE=${AGENT_IMAGE:-ubuntu:14.04}

export CATTLE_ADDRESS
export CATTLE_AGENT_CONNECT
export CATTLE_INTERNAL_ADDRESS
export CATTLE_NODE_NAME
export CATTLE_ROLE
export CATTLE_SERVER
export CATTLE_TOKEN

while true; do
    case "$1" in
        -d | --debug)                   DEBUG=true                 ;;
        -s | --server)           shift; CATTLE_SERVER=$1           ;;
        -t | --token)            shift; CATTLE_TOKEN=$1            ;;
        -c | --ca-checksum)      shift; CATTLE_CA_CHECKSUM=$1      ;;
        -a | --all-roles)               ALL=true                   ;;
        -e | --etcd)                    ETCD=true                  ;;
        -w | --worker)                  WORKER=true                ;;
        -p | --controlplane)            CONTROL=true               ;;
        -n | --node-name)        shift; CATTLE_NODE_NAME=$1        ;;
        -r | --no-register)             CATTLE_AGENT_CONNECT=true  ;;
        -a | --address)          shift; CATTLE_ADDRESS=$1          ;;
        -i | --internal-address) shift; CATTLE_INTERNAL_ADDRESS=$1 ;;
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

export CATTLE_ADDRESS=$(get_address $CATTLE_ADDRESS)
export CATTLE_INTERNAL_ADDRESS=$(get_address $CATTLE_INTERNAL_ADDRESS)

if [ -z "$CATTLE_ADDRESS" ]; then
    # Example output: '8.8.8.8 via 10.128.0.1 dev ens4 src 10.128.0.34 uid 0'
    CATTLE_ADDRESS=$(ip -o route get 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')
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
    if [ ! -s $temp ]; then
      error "Failed to pull the cacert from the rancher server settings at $CATTLE_SERVER/v3/settings/cacerts"
      exit 1
    fi
    CATTLE_SERVER_CHECKSUM=$(sha256sum $temp | awk '{print $1}')
    if [ $CATTLE_SERVER_CHECKSUM != $CATTLE_CA_CHECKSUM ]; then
        rm -f $temp
        error "Configured cacerts checksum ($CATTLE_SERVER_CHECKSUM) does not match given --ca-checksum ($CATTLE_CA_CHECKSUM)"
        error "Please check if the correct certificate is configured at $CATTLE_SERVER/v3/settings/cacerts"
        exit 1
    else
        mkdir -p /etc/kubernetes/ssl/certs
        mv $temp /etc/kubernetes/ssl/certs/serverca
    fi
fi

if [ -z "$CATTLE_SERVER" ]; then
    error -- --server is a required option
    exit 1
fi

if [ "$CATTLE_K8S_MANAGED" != "true" ]; then
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
