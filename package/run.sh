#!/bin/bash
set -e

# Logging helper functions
info()
{
    echo "INFO:" "$@" 1>&2
}

error()
{
    echo "ERROR:" "$@" 1>&2
}
warn()
{
    echo "WARN:" "$@" 1>&2
}

# Print all given arguments
if [ $# -ne 0 ]; then
    info "Arguments: $(echo $@ | sed -e 's/\(token\s\)\w*/\1REDACTED/')"
fi

if [ "$1" = "--" ]; then
    shift 1
    exec "$@"
fi

if [ "$CLUSTER_CLEANUP" = true ]; then
    exec agent
fi

#checking if aws metadata is available or not  # Setting TOKEN as global var to be overwritten and used in get_address func 
IMD_TOKEN="" 
check_aws_metadata() 
{     
      local IMD=`curl -s -o /dev/null -w "%{http_code}" http://169.254.169.254/`
      if [ $IMD -eq 200 ]
      then
        IMD_TOKEN=`curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600"`
        echo "TOKEN: $IMD_TOKEN"     
      elif [ $IMD -eq 403 ]
      then
        echo "Error: Aws Instance metadata is disabled"
        exit 1     
      elif [ $IMD -eq 401 ]
      then
        echo "INFO: Auth error, Instance metatdata version 2 (IMDV2) is set to required: Setting TOKEN Value"
        IMD_TOKEN=`curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600"`
        echo "TOKEN: $IMD_TOKEN"
      else
        echo "Error: Unknwon error: Status Code $IMD"
        exit 1
      fi
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
                check_aws_metadata
                echo $(curl -s http://169.254.169.254/latest/meta-data/local-ipv4)
                ;;
            awspublic)
                check_aws_metadata
                echo $(curl  -H "X-aws-ec2-metadata-token: $IMD_TOKEN" -s http://169.254.169.254/latest/meta-data/public-ipv4)
                ;;
            doprivate)
                echo $(curl -H "X-aws-ec2-metadata-token: $IMD_TOKEN" -s http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address)
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

check_url()
{
    local url=$1
    local err
    err=$(curl --insecure -sS -fL -o /dev/null --stderr - $url | head -n1 ; exit ${PIPESTATUS[0]})
    if [ $? -eq 0 ]
    then
        echo ""
    else
        echo ${err} | sed -e 's/^curl: ([0-9]\+) //'
    fi
}

check_x509_cert()
{
    local cert=$1
    local err
    err=$(openssl x509 -in $cert -noout 2>&1)
    if [ $? -eq 0 ]
    then
        echo ""
    else
        echo ${err}
    fi
}

export CATTLE_ADDRESS
export CATTLE_AGENT_CONNECT
export CATTLE_INTERNAL_ADDRESS
export CATTLE_NODE_NAME
export CATTLE_ROLE
export CATTLE_SERVER
export CATTLE_TOKEN
export CATTLE_NODE_LABEL
export CATTLE_WRITE_CERT_ONLY
export CATTLE_NODE_TAINTS

while true; do
    case "$1" in
        -d | --debug)                   DEBUG=true                  ;;
        -s | --server)           shift; CATTLE_SERVER=$1            ;;
        -t | --token)            shift; CATTLE_TOKEN=$1             ;;
        -c | --ca-checksum)      shift; CATTLE_CA_CHECKSUM=$1       ;;
        -a | --all-roles)               ALL=true                    ;;
        -e | --etcd)                    ETCD=true                   ;;
        -w | --worker)                  WORKER=true                 ;;
        -p | --controlplane)            CONTROL=true                ;;
        -n | --node-name)        shift; CATTLE_NODE_NAME=$1         ;;
        -r | --no-register)             CATTLE_AGENT_CONNECT=true   ;;
        -a | --address)          shift; CATTLE_ADDRESS=$1           ;;
        -i | --internal-address) shift; CATTLE_INTERNAL_ADDRESS=$1  ;;
        -l | --label)            shift; CATTLE_NODE_LABEL+=",$1"    ;;
        -o | --only-write-certs)        CATTLE_WRITE_CERT_ONLY=true ;;
        --taints)                shift; CATTLE_NODE_TAINTS+=",$1"   ;;
        *) break;
    esac
    shift
done

if [ "$DEBUG" = true ]; then
    set -x
fi

if [ "$CATTLE_CLUSTER" != "true" ]; then
    if [ ! -w /var/run/docker.sock ] || [ ! -S /var/run/docker.sock ]; then
        warn "Docker socket is not available!"
        warn "Please bind mount in the docker socket to /var/run/docker.sock if docker errors occur"
        warn "example:  docker run -v /var/run/docker.sock:/var/run/docker.sock ..."
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

if [ -z "$CATTLE_SERVER" ]; then
    error -- --server is a required option
    exit 1
fi

info "Environment: $(echo $(printenv | grep CATTLE | sort | sed -e 's/\(CATTLE_TOKEN=\).*/\1REDACTED/'))"
info "Using resolv.conf: $(echo $(cat /etc/resolv.conf | grep -v ^#))"
if grep -E -q '^nameserver 127.*.*.*|^nameserver localhost|^nameserver ::1' /etc/resolv.conf; then
    warn "Loopback address found in /etc/resolv.conf, please refer to the documentation how to configure your cluster to resolve DNS properly"
fi

CATTLE_SERVER_PING="${CATTLE_SERVER}/ping"
err=$(check_url $CATTLE_SERVER_PING)
if [[ $err ]]; then
    error "${CATTLE_SERVER_PING} is not accessible (${err})"
    exit 1
else
    info "${CATTLE_SERVER_PING} is accessible"
fi

# Extract hostname from URL
CATTLE_SERVER_HOSTNAME=$(echo $CATTLE_SERVER | sed -e 's/[^/]*\/\/\([^@]*@\)\?\([^:/]*\).*/\2/')
CATTLE_SERVER_HOSTNAME_WITH_PORT=$(echo $CATTLE_SERVER | sed -e 's/[^/]*\/\/\([^@]*@\)\?\(.*\).*/\2/')
# Resolve IPv4 address(es) from hostname
RESOLVED_ADDR=$(getent ahostsv4 $CATTLE_SERVER_HOSTNAME | sed -n 's/ *STREAM.*//p')

# If CATTLE_SERVER_HOSTNAME equals RESOLVED_ADDR, its an IP address and there is nothing to resolve
if [ "${CATTLE_SERVER_HOSTNAME}" != "${RESOLVED_ADDR}" ]; then
  info "${CATTLE_SERVER_HOSTNAME} resolves to $(echo $RESOLVED_ADDR)"
fi

if [ -n "$CATTLE_CA_CHECKSUM" ]; then
    temp=$(mktemp)
    curl --insecure -s -fL $CATTLE_SERVER/v3/settings/cacerts | jq -r '.value | select(length > 0)' > $temp
    if [ ! -s $temp ]; then
      error "The environment variable CATTLE_CA_CHECKSUM is set but there is no CA certificate configured at $CATTLE_SERVER/v3/settings/cacerts"
      exit 1
    fi
    err=$(check_x509_cert $temp)
    if [[ $err ]]; then
        error "Value from $CATTLE_SERVER/v3/settings/cacerts does not look like an x509 certificate (${err})"
        error "Retrieved cacerts:"
        cat $temp
        exit 1
    else
        info "Value from $CATTLE_SERVER/v3/settings/cacerts is an x509 certificate"
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
        chmod 755 /etc/kubernetes/ssl
        chmod 700 /etc/kubernetes/ssl/certs
        chmod 600 /etc/kubernetes/ssl/certs/serverca
        mkdir -p /etc/docker/certs.d/$CATTLE_SERVER_HOSTNAME_WITH_PORT
        cp /etc/kubernetes/ssl/certs/serverca /etc/docker/certs.d/$CATTLE_SERVER_HOSTNAME_WITH_PORT/ca.crt

    fi
fi

exec tini -- agent

