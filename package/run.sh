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
    export CATTLE_ENTRYPOINT_BYPASS=true
    shift 1
    exec "$@"
fi

if [ "$CLUSTER_CLEANUP" = true ]; then
    export CATTLE_ENTRYPOINT_BYPASS=true
    exec agent
fi

export CATTLE_AGENT_CONNECT
export CATTLE_NODE_NAME
export CATTLE_ROLE
export CATTLE_SERVER
export CATTLE_TOKEN
export CATTLE_NODE_LABEL
export CATTLE_WRITE_CERT_ONLY
export CATTLE_NODE_TAINTS
export CATTLE_CA_CHECKSUM

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
        --address)               shift; ;; # deprecated
        -i | --internal-address) shift; ;; # deprecated
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

if [ "$CATTLE_K8S_MANAGED" != "true" ]; then
    if [ -z "$CATTLE_TOKEN" ]; then
        error -- --token is a required option
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

exec tini -- agent

