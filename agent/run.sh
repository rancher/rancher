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
# End copy

check_debug

launch_volume()
{
    if docker inspect rancher-agent-state >/dev/null 2>&1; then
        return
    fi

    local opts=""

    if [ "${CATTLE_VAR_LIB_WRITABLE}" = "true" ]; then
        opts="-v /var/lib/rancher:/var/lib/rancher"
    else
        opts="-v /var/lib/rancher"
    fi

    docker run \
        --name rancher-agent-state \
        -v /var/lib/cattle \
        -v /var/log/rancher:/var/log/rancher \
        ${opts} ${RANCHER_AGENT_IMAGE} state
}

inspect_host()
{
    docker run --rm --privileged -v /run:/run -v /var/lib:/var/lib ${RANCHER_AGENT_IMAGE} inspect-host
}

launch_agent()
{
    launch_volume

    local var_lib_docker=$(resolve_var_lib_docker)

    docker run \
        -d \
        --name rancher-agent \
        --restart=always \
        --net=host \
        --pid=host \
        --privileged \
        -e CATTLE_AGENT_PIDNS=host \
        -e http_proxy \
        -e https_proxy \
        -e NO_PROXY \
        -e CATTLE_PHYSICAL_HOST_UUID \
        -e CATTLE_SCRIPT_DEBUG \
        -e CATTLE_ACCESS_KEY \
        -e CATTLE_SECRET_KEY \
        -e CATTLE_AGENT_IP \
        -e CATTLE_HOST_API_PROXY \
        -e CATTLE_SYSTEMD \
        -e CATTLE_URL \
        -e CATTLE_HOST_LABELS \
        -e CATTLE_VOLMGR_ENABLED \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /lib/modules:/lib/modules:ro \
        -v ${var_lib_docker}:${var_lib_docker} \
        -v /proc:/host/proc \
        -v /dev:/host/dev \
        --volumes-from rancher-agent-state \
        "${RANCHER_AGENT_IMAGE}" "$@"
}

resolve_var_lib_docker()
{
    local dir="$(docker inspect -f '{{index .Volumes "/var/lib/cattle"}}' rancher-agent-state)"
    echo $(dirname $(dirname $(dirname $dir)))
}

verify_docker_client_server_version()
{
    local client_version=$(docker version |grep Client\ version | cut -d":" -f2)
    info "Checking for Docker version >=" $client_version
    docker version 2>&1 | grep Server\ version >/dev/null || {
        echo "Please ensure Host Docker version is >=${client_version} and container has r/w permissions to docker.sock" 1>&2
        exit 1
    }
    info Found $(docker version 2>&1 | grep Server\ version)
    for i in version info; do
        docker $i | while read LINE; do
            info "docker $i:" $LINE
        done
    done
}

delete_container()
{
    while docker inspect $1 >/dev/null 2>&1; do
        info Deleting container $1
        docker rm -f $1 >/dev/null 2>&1 || true
    done
}

cleanup_agent()
{
    delete_container rancher-agent
}

cleanup_upgrade()
{
    delete_container rancher-agent-upgrade
}

setup_state()
{
    mkdir -p /var/lib/{cattle,rancher/state}

    export CATTLE_STATE_DIR=/var/lib/rancher/state
    export CATTLE_AGENT_LOG_FILE=/var/log/rancher/agent.log
    export CATTLE_CADVISOR_WRAPPER=cadvisor.sh

    if [ "$CATTLE_SYSTEMD" = "true" ]; then
        mkdir -p /run/systemd/system
    fi
}

load()
{
    local content=$(curl -sL $1)

    if [[ "$content" =~ .!/bin/sh.* ]]; then
        eval "$content"
    fi
}

print_token()
{
    local token_file=/var/lib/rancher/state/.registration_token
    local token=

    if [ -e $token_file ]; then
        token="$(<$token_file)"
    fi

    if [ -z "$token" ]; then
        token=$(openssl rand -hex 64)
        mkdir -p $(dirname $token_file)
        echo $token > $token_file
    fi

    info env "TOKEN=$token"
}

register()
{
    ENV=$(./register.py $TOKEN)
    eval "$ENV"

    export CATTLE_AGENT_IP=${CATTLE_AGENT_IP:-${DETECTED_CATTLE_AGENT_IP}}
}

run_bootstrap()
{
    SCRIPT=/tmp/bootstrap.sh
    touch $SCRIPT
    chmod 700 $SCRIPT

    export CATTLE_CONFIG_URL="${CATTLE_CONFIG_URL:-${CATTLE_URL}}"
    export CATTLE_STORAGE_URL="${CATTLE_STORAGE_URL:-${CATTLE_URL}}"

    curl -u ${CATTLE_ACCESS_KEY}:${CATTLE_SECRET_KEY} -s ${CATTLE_URL}/scripts/bootstrap > $SCRIPT 

    # Sanity check if this account is really being authenticated as an agent account or the default admin auth
    if curl -f -u ${CATTLE_ACCESS_KEY}:${CATTLE_SECRET_KEY} -s ${CATTLE_URL}/schemas/account >/dev/null 2>&1; then
        error Please re-register this agent
        exit 1
    fi

    info "Starting agent for ${CATTLE_ACCESS_KEY}"
    if [ "$CATTLE_EXEC_AGENT" = "true" ]; then
        exec bash $SCRIPT "$@"
    else
        bash $SCRIPT "$@"
    fi
}

run()
{
    mount --rbind /host/dev /dev
    while true; do
        run_bootstrap "$@" || true
        sleep 5
    done
}

read_rancher_agent_env()
{
    info Reading environment from rancher-agent
    local save=$RANCHER_AGENT_IMAGE
    eval $(docker inspect rancher-agent | jq -r '"export \"" + .[0].Config.Env[] + "\""')
    RANCHER_AGENT_IMAGE=$save
}

print_url()
{
    local url=$(echo "${CATTLE_URL}"| sed -e 's!/v1/scripts.*!/v1!')
    echo $url
}

wait_for()
{
    local url="$(print_url $CATTLE_URL)"
    info "Attempting to connect to: ${url}"
    for ((i=0; i < 300; i++)); do
        if ! curl -f -s ${CATTLE_URL} >/dev/null 2>&1; then
            error ${url} is not accessible
            sleep 2
            if [ "$i" -eq "299" ]; then
                error "Could not reach ${url}. Giving up."
                exit 1
            fi
        else
            info "${url} is accessible"
            break
        fi
    done
}

inspect()
{
    print_token

    if mkdir -p /var/lib/rancher/state >/dev/null 2>&1; then
        info env "CATTLE_VAR_LIB_WRITABLE=true"
    else
        info env "CATTLE_VAR_LIB_WRITABLE=false"
    fi

    if [ -e /var/run/system-docker.sock ]; then
        info env "CATTLE_RANCHEROS=true"
    else
        info env "CATTLE_RANCHEROS=false"
    fi

    if [ -e /run/systemd/system ]; then
        info env "CATTLE_SYSTEMD=true"
    else
        info env "CATTLE_SYSTEMD=false"
    fi
}

setup_env()
{
    if [ "$1" != "upgrade" ]; then
        local env="$(./resolve_url.py $CATTLE_URL)"
        load "$env"
    fi

    info Inspecting host capabilities
    local content=$(inspect_host)

    echo "$content" | grep -v 'INFO: env' || true
    eval $(echo "$content" | grep 'INFO: env' | sed 's/INFO: env//g')

    export CATTLE_SYSTEMD

    info System: ${CATTLE_SYSTEMD}
    info Host writable: ${CATTLE_VAR_LIB_WRITABLE}
    info Token: $(echo $TOKEN | sed 's/........*/xxxxxxxx/g')

    if [[ -z "$CATTLE_ACCESS_KEY" || -z "$CATTLE_SECRET_KEY" ]]; then
        info Running registration
        register
    else
        info Skipping registration
    fi

    info Printing Environment
    env | sort | while read LINE; do
        if [[ $LINE =~ RANCHER.* || $LINE =~ CATTLE.* ]]; then
            info "ENV:" $(echo $LINE | sed 's/\(SECRET.*=\).*/\1xxxxxxx/g')
        fi
    done
}

setup_cattle_url()
{
    if [ "$1" = "register" ]; then
        if [ -z "$RANCHER_URL" ]; then
            info No RANCHER_URL environment variable, exiting
            exit 0
        fi
        CATTLE_URL="$RANCHER_URL"
    elif [ "$1" = "upgrade" ]; then
        read_rancher_agent_env
    else
        CATTLE_URL="$1"
    fi

    if echo $CATTLE_URL | grep -qE '127.0.0.1|localhost'; then
        local gateway=$(docker run --rm --net=host $RANCHER_AGENT_IMAGE -- ip route get 8.8.8.8 | grep via | awk '{print $7}')
        CATTLE_URL=$(echo $CATTLE_URL | sed -e 's/127.0.0.1/'$gateway'/' -e 's/localhost/'$gateway'/')
    fi

    export CATTLE_URL
}


if [ "$#" == 0 ]; then
    error "One parameter required"
    exit 1
fi

if [[ $1 =~ http.* || $1 = "register" || $1 = "upgrade" ]]; then
    echo $http_proxy $https_proxy
    setup_cattle_url $1
    if [ "$1" = "upgrade" ]; then
        info Running upgrade
    else
        info Running Agent Registration Process, CATTLE_URL=$(print_url $CATTLE_URL)
    fi
    verify_docker_client_server_version
    if [ "$1" != "upgrade" ]; then
        wait_for
    fi
    setup_env $1
    cleanup_agent
    ID=$(launch_agent run)
    info Launched Rancher Agent: $ID
elif [ "$1" = "inspect-host" ]; then
    inspect
elif [ "$1" = "state" ]; then
    echo Rancher State
elif [ "$1" = "run" ]; then
    cleanup_upgrade
    setup_state
    run
elif [ "$1" = "--" ]; then
    shift 1
    exec "$@"
fi
