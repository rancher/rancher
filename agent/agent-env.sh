#!/bin/bash
set -e

trap "exit 1" SIGINT SIGTERM

# This is copied from common/scripts.sh, if there is a change here
# make it in common and then copy here
check_debug()
{
    if [ -n "$CATTLE_SCRIPT_DEBUG" ] || echo "${@}" | grep -q -- --debug; then
        export CATTLE_SCRIPT_DEBUG=true
        export PS4='[${BASH_SOURCE##*/}:${LINENO}] '
        set -x
    fi
}

info()
{
    echo "INFO:" "${@}"
}

error()
{
    echo "ERROR:" "${@}" 1>&2
}

export CATTLE_HOME=${CATTLE_HOME:-/var/lib/cattle}

check_debug
# End copy

export CATTLE_RUN_REGISTRATION=false
export CATTLE_INSIDE_DOCKER=true

ETCD_DEFAULT="http://127.0.0.1:4001/v2/keys"
ETCD_URL=${ETCD_URL:-$ETCD_DEFAULT}

get_register_env()
{
    eval "$(curl -s ${CATTLE_REGISTRATION_URL})"
}

register()
{
    local host_token_file=/var/lib/cattle/.registration_token
    local token_file=${token_file:-/host${host_token_file}}
    local token=

    mkdir -p /host/${CATTLE_HOME}

    if [ -e $token_file ]; then
        token="$(<$token_file)"
    fi

    if [ -z "$token" ]; then
        token=$(openssl rand -hex 64)
        mkdir -p $(dirname $token_file)
        echo $token > $token_file

        echo "Created $host_token_file, do not delete it.  It identifies this server with the core system."
    fi

    ENV=$(./register.py $token)
    eval "$ENV"
}

wait_for_url()
{
    local url=$1
    local first=true

    while ! curl -L --fail -s $url; do
        if [ "$first" = "true" ]; then
            echo "Waiting for $url"
        fi
        sleep 5
    done
}

etcd_registration()
{
    local url="$ETCD_URL/keys/rancher/registration_url"
    wait_for_url $url

    CATTLE_REGISTRATION_URL="$(curl -L -s $url | jq -r .node.value)"
}

resolve_url()
{
    if [ "$CATTLE_URL_ARG" = "etcd" ]; then
        local url="$ETCD_URL/keys/services/rancher-server"
        wait_for_url $url

        while true; do
            CATTLE_URL_ARG="$(curl -s $url | jq -r '.node.nodes[0].value')"
            if [[ "$CATTLE_URL_ARG" = "null" || -z "$CATTLE_URL_ARG" ]]; then
                echo "Waiting for service to register at $url"
                sleep 5
            else
                CATTLE_URL_ARG="http://${CATTLE_URL_ARG}"
                break
            fi
        done
    fi

    local temp=$(./resolve_url.py "$CATTLE_URL_ARG")
    if [ -n "${temp}" ]; then
        CATTLE_REGISTRATION_URL="$temp"
    fi
}

setup_env()
{
    while [ "$#" -gt 0 ]; do
        case $1 in
        -*)
            continue
            ;;
        *)
            exec "$@"
            ;;
        esac
        shift 1
    done

    if [[ -n "$CATTLE_URL_ARG" ]]; then
        resolve_url
    fi

    if [[ -n "$CATTLE_ETCD_REGISTRATION" ]]; then
        etcd_registration
    fi

    if [[ -n "$CATTLE_REGISTRATION_URL" ]]; then
        get_register_env
    fi

    if [[ -n "$CATTLE_URL" && -n "$CATTLE_REGISTRATION_ACCESS_KEY" && "$CATTLE_REGISTRATION_SECRET_KEY" ]]; then
        register
    fi

    if ! [[ -n "$CATTLE_URL" && -n "$CATTLE_ACCESS_KEY" && "$CATTLE_SECRET_KEY" ]]; then
        echo 'Invalid environment, maybe the server is inaccessible' 1>&2
        sleep 5
        exit 1
    fi

    export CATTLE_CONFIG_URL="${CATTLE_CONFIG_URL:-${CATTLE_URL}}"
    export CATTLE_STORAGE_URL="${CATTLE_STORAGE_URL:-${CATTLE_URL}}"
}

setup_mounts()
{
    if [ "$CATTLE_LIBVIRT_ENABLE" = "true" ]; then
        mkdir -p /host/run/cattle/libvirt/libvirt
        mkdir -p /host/var/lib/cattle/libvirt
        mkdir -p /run/libvirt
        mkdir -p /var/lib/cattle/libvirt
        mkdir -p /var/lib/libvirt
    fi

    mkdir -p /var/lib/cattle
    mount --rbind /host/var/lib/cattle /var/lib/cattle

    for i in /var/lib/docker /lib/modules; do
        local host_dir=/host${i}

        if [ -e $host_dir ]; then
            mkdir -p $i
            mount --bind $host_dir $i
        fi
    done

    for i in /host/run/docker.sock /host/var/run/docker.sock; do
        if [ -e $i ] && [ ! -e /run/docker.sock ]; then
            ln -s $i /run/docker.sock
            break
        fi
    done


    if [ "$CATTLE_LIBVIRT_ENABLE" = "true" ]; then
        mount --bind /host/run/cattle/libvirt/libvirt /run/libvirt
        mount --bind /var/lib/cattle/libvirt /var/lib/libvirt
    fi
}

run_bootstrap()
{
    SCRIPT=/tmp/bootstrap.sh
    touch $SCRIPT
    chmod 700 $SCRIPT

    curl -u ${CATTLE_ACCESS_KEY}:${CATTLE_SECRET_KEY} -s ${CATTLE_URL}/scripts/bootstrap > $SCRIPT 
    echo "echo INFO: Starting for agent ${CATTLE_ACCESS_KEY}"
    if [ "$CATTLE_EXEC_AGENT" = "true" ]; then
        exec bash $SCRIPT "$@"
    else
        bash $SCRIPT "$@"
    fi
}

setup_env "$@"
setup_mounts

while true; do
    run_bootstrap "$@" || true
    sleep 2
done
