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

    if [ "$(var_lib_writable)" = "true" ]; then
        opts="-v /var/lib/rancher:/var/lib/rancher"
    fi

    if [ "$(var_lib_cattle)" = "true" ]; then
        opts="$opts -v /var/lib/cattle:/var/lib/cattle-legacy"
    fi

    docker run \
        --name rancher-agent-state \
        -v /var/lib/cattle \
        -v /var/log/rancher:/var/log/rancher \
        ${opts} ${CATTLE_AGENT_IMAGE} state
}

var_lib_writable()
{
    docker run --rm --privileged -v /var/lib:/var/lib ${CATTLE_AGENT_IMAGE} check-var-lib
}

var_lib_cattle()
{
    docker run --rm --privileged -v /var/lib:/var/lib ${CATTLE_AGENT_IMAGE} check-var-lib-cattle
}

launch_agent()
{
    launch_volume

    local var_lib_docker=$(resolve_var_lib_docker)

    docker run \
        ${DOCKER_OPTS} \
        --privileged \
        -e CATTLE_PHYSICAL_HOST_UUID=${CATTLE_PHYSICAL_HOST_UUID} \
        -e CATTLE_SCRIPT_DEBUG=${CATTLE_SCRIPT_DEBUG} \
        -e CATTLE_ACCESS_KEY="${CATTLE_ACCESS_KEY}" \
        -e CATTLE_SECRET_KEY="${CATTLE_SECRET_KEY}" \
        -e CATTLE_AGENT_IP="${CATTLE_AGENT_IP}" \
        -e CATTLE_URL="${CATTLE_URL}" \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /lib/modules:/lib/modules:/ro \
        -v ${var_lib_docker}:${var_lib_docker} \
        -v /proc:/host/proc \
        --volumes-from rancher-agent-state \
        "${CATTLE_AGENT_IMAGE}" "$@"
}

resolve_var_lib_docker()
{
    local dir="$(docker inspect -f '{{index .Volumes "/var/lib/cattle"}}' rancher-agent-state)"
    echo $(dirname $(dirname $(dirname $dir)))
}

verify_docker_client_server_version()
{
    docker version 2>&1 | grep Server\ version >/dev/null || {
        local client_version=$(docker version |grep Client\ version | cut -d":" -f2)
        echo "Please ensure Host Docker version is >=${client_version} and container has r/w permissions to docker.sock" 1>&2
        exit 1
    }
}

resolve_image()
{
    local image=$(docker inspect -f '{{.Image}}' $(hostname))

    if [ -z "$image" ]; then
        image=${RANCHER_AGENT_IMAGE:-rancher/agent:latest}
    else
        local gateway=$(docker run --rm --net=host $image -- ip route get 8.8.8.8 | grep via | awk '{print $7}')
        CATTLE_URL=$(echo $CATTLE_URL | sed -e 's/127.0.0.1/'$gateway'/' -e 's/localhost/'$gateway'/')
    fi

    CATTLE_AGENT_IMAGE=${CATTLE_AGENT_IMAGE:-$image}
}

delete_container()
{
    while docker inspect $1 >/dev/null 2>&1; do
        docker rm -f $1 >/dev/null 2>&1 || true
        sleep 1
    done
}

cleanup_agent()
{
    delete_container rancher-agent
}

cleanup_upgrade()
{
    delete_container rancher-agent-upgrade
    delete_container rancher-agent-upgrade-stage2
}

setup_state()
{
    mkdir -p /var/lib/{cattle,rancher/state}

    if [[ -e /var/lib/rancher/state && ! -e /var/lib/cattle/state ]]; then
        ln -s /var/lib/rancher/state /var/lib/cattle/state
    fi

    if [[ ! -e /var/lib/cattle/logs ]]; then
        mkdir -p /var/log/rancher
        ln -s /var/log/rancher /var/lib/cattle/logs
    fi

    for i in .docker_uuid .physical_host_uuid .registration_token; do
        if [[ ! -e /var/lib/rancher/state/$i && -e /var/lib/cattle-legacy/$i ]]; then
            cp -v /var/lib/cattle-legacy/$i /var/lib/rancher/state/$i
        fi
    done

    export CATTLE_STATE_DIR=/var/lib/cattle/state
    export CATTLE_AGENT_LOG_FILE=/var/lib/cattle/logs/agent.log
    export CATTLE_CADVISOR_WRAPPER=cadvisor.sh
}

load()
{
    local content=$(curl -sL $1)

    if [[ "$content" =~ .!/bin/sh.* ]]; then
        eval "$content"
    fi
}

register()
{
    load $(./resolve_url.py $CATTLE_URL)

    local legacy_token_file=/var/lib/cattle-legacy/.registration_token
    local token_file=/var/lib/cattle/state/.registration_token
    local token=

    if [[ -e ${legacy_token_file} && ! -e ${token_file} ]]; then
        cp -f ${legacy_token_file} ${token_file}
    fi

    if [ -e $token_file ]; then
        token="$(<$token_file)"
    fi

    if [ -z "$token" ]; then
        token=$(openssl rand -hex 64)
        mkdir -p $(dirname $token_file)
        echo $token > $token_file
    fi

    ENV=$(./register.py $token)
    eval "$ENV"

    CATTLE_AGENT_IP=${CATTLE_AGENT_IP:-${DETECTED_CATTLE_AGENT_IP}}
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
        resolve_image
        DOCKER_OPTS="--rm" launch_agent ${CATTLE_URL}
        exit 0
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
    while true; do
        run_bootstrap "$@" || true
        sleep 5
    done
}

upgrade()
{
    eval $(docker inspect rancher-agent | jq -r '"export \"" + .[0].Config.Env[] + "\""')
}

wait_for()
{
    for ((i=0; i < 300; i++)); do
        if ! curl -f -s ${CATTLE_URL} >/dev/null 2>&1; then
            error "${CATTLE_URL}" is not accessible
        else
            break
        fi
    done
}

DOCKER_OPTS="-d --name rancher-agent --restart=always --net=host"

if [ "$#" == 0 ]; then
    error "One parameter required"
    exit 1
fi

if [[ $1 =~ http.* ]]; then
    CATTLE_URL="$1"
    verify_docker_client_server_version
    resolve_image
    wait_for
    cleanup_agent
    DOCKER_OPTS="--rm" launch_agent register
elif [ "$1" = "check-var-lib" ]; then
    if mkdir -p /var/lib/rancher/state >/dev/null 2>&1; then
        echo true
    else
        echo false
    fi
elif [ "$1" = "check-var-lib-cattle" ]; then
    if [ -d /var/lib/cattle ]; then
        echo true
    else
        echo false
    fi
elif [ "$1" = "state" ]; then
    echo Rancher State
elif [ "$1" = "register" ]; then
    verify_docker_client_server_version
    resolve_image
    cleanup_agent
    setup_state
    register
    launch_agent run
elif [ "$1" = "upgrade" ]; then
    verify_docker_client_server_version
    resolve_image
    upgrade
    DOCKER_OPTS="--rm --name rancher-agent-upgrade-stage2" launch_agent upgrade-stage2
elif [ "$1" = "upgrade-stage2" ]; then
    verify_docker_client_server_version
    resolve_image
    cleanup_agent
    setup_state
    launch_agent run
elif [ "$1" = "run" ]; then
    cleanup_upgrade
    setup_state
    run
elif [ "$1" = "--" ]; then
    shift 1
    exec "$@"
fi
