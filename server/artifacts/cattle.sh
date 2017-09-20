#!/bin/bash
set -e -x

cd /var/lib/cattle

SCRIPT=/usr/share/cattle/bin/cattle
HASH=$(md5sum $SCRIPT | awk '{print $1}')
LOG_DIR=/var/lib/cattle/logs
export S6_SERVICE_DIR=${S6_SERVICE_DIR:-$S6_SERVICE_DIR}

setup_local_agents()
{
    if [ "${CATTLE_USE_LOCAL_ARTIFACTS}" == "true" ]; then
        if [ -f /usr/share/cattle/env_vars ]; then
            source /usr/share/cattle/env_vars
        fi
    fi
}

setup_graphite()
{
    # Setup Graphite
    export CATTLE_GRAPHITE_HOST=${CATTLE_GRAPHITE_HOST:-$GRAPHITE_PORT_2003_TCP_ADDR}
    export CATTLE_GRAPHITE_PORT=${CATTLE_GRAPHITE_PORT:-$GRAPHITE_PORT_2003_TCP_PORT}
}

setup_prometheus()
{
    # Setup Prometheus Graphite exporter
    if [ "${CATTLE_PROMETHEUS_EXPORTER}" == "true" ]; then
        s6-svc -u ${S6_SERVICE_DIR}/graphite_exporter
        export DEFAULT_CATTLE_GRAPHITE_HOST=127.0.0.1
        export DEFAULT_CATTLE_GRAPHITE_PORT=9109
    fi
}

setup_gelf()
{
    # Setup GELF
    export CATTLE_LOGBACK_OUTPUT_GELF_HOST=${CATTLE_LOGBACK_OUTPUT_GELF_HOST:-$GELF_PORT_12201_UDP_ADDR}
    export CATTLE_LOGBACK_OUTPUT_GELF_PORT=${CATTLE_LOGBACK_OUTPUT_GELF_PORT:-$GELF_PORT_12201_UDP_PORT}
    if [ -n "$CATTLE_LOGBACK_OUTPUT_GELF_HOST" ]; then
        export CATTLE_LOGBACK_OUTPUT_GELF=${CATTLE_LOGBACK_OUTPUT_GELF:-true}
    fi
}

setup_mysql()
{
    # Set in the Dockerfile by default... overriden by runtime.
    if [ ${CATTLE_DB_CATTLE_DATABASE} == "mysql" ]; then
        export CATTLE_DB_CATTLE_MYSQL_HOST=${CATTLE_DB_CATTLE_MYSQL_HOST:-$MYSQL_PORT_3306_TCP_ADDR}
        export CATTLE_DB_CATTLE_MYSQL_PORT=${CATTLE_DB_CATTLE_MYSQL_PORT:-$MYSQL_PORT_3306_TCP_PORT}
        export CATTLE_DB_CATTLE_USERNAME=${CATTLE_DB_CATTLE_USERNAME:-cattle}
        export CATTLE_DB_CATTLE_PASSWORD=${CATTLE_DB_CATTLE_PASSWORD:-cattle}
        export CATTLE_DB_CATTLE_MYSQL_NAME=${CATTLE_DB_CATTLE_MYSQL_NAME:-cattle}

        if [ -z "$CATTLE_DB_CATTLE_MYSQL_HOST" ]; then
            export CATTLE_DB_CATTLE_MYSQL_HOST="localhost"
            /usr/share/cattle/mysql.sh
        fi

        if [ -z "$CATTLE_DB_CATTLE_MYSQL_PORT" ]; then
            CATTLE_DB_CATTLE_MYSQL_PORT=3306
        fi
    fi
}

setup_proxy()
{
    if [ -n "$http_proxy" ]; then
        local host=$(echo $http_proxy | sed 's!.*//!!' | cut -f1 -d:)
        local port=$(echo $http_proxy | sed 's!.*//!!' | cut -f2 -d:)

        PROXY_ARGS="-Dhttp.proxyHost=${host}"
        if [ "$host" != "$port" ]; then
            PROXY_ARGS="$PROXY_ARGS -Dhttp.proxyPort=${port}"
        fi
    fi

    if [ -n "$https_proxy" ]; then
        local host=$(echo $https_proxy | sed 's!.*//!!' | cut -f1 -d:)
        local port=$(echo $https_proxy | sed 's!.*//!!' | cut -f2 -d:)

        PROXY_ARGS="$PROXY_ARGS -Dhttps.proxyHost=${host}"
        if [ "$host" != "$port" ]; then
            PROXY_ARGS="$PROXY_ARGS -Dhttps.proxyPort=${port}"
        fi
    fi
}

run() {
    setup_local_agents
    setup_graphite
    setup_prometheus
    setup_gelf
    setup_mysql
    setup_proxy

    env | grep CATTLE | grep -v PASS | sort

    local ram=$(free -g --si | awk '/^Mem:/{print $2}')
    if [ ${ram} -gt 6 ]; then
        MX="4g"
    elif [ ${ram} -gt 2 ]; then
        MX="2g"
    else
        MX="1g"
    fi

    unset DEFAULT_CATTLE_API_UI_JS_URL
    unset DEFAULT_CATTLE_API_UI_CSS_URL
    export JAVA_OPTS="${CATTLE_JAVA_OPTS:--XX:+UseConcMarkSweepGC -XX:+CMSClassUnloadingEnabled -Xms128m -Xmx${MX} -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=$LOG_DIR} $PROXY_ARGS $JAVA_OPTS"
    exec $SCRIPT "$@" $ARGS
}

master()
{
    unset CATTLE_API_UI_URL
    unset CATTLE_CATTLE_VERSION
    unset CATTLE_RANCHER_SERVER_VERSION
    unset CATTLE_RANCHER_SERVER_VERSION
    unset CATTLE_USE_LOCAL_ARTIFACTS
    unset DEFAULT_CATTLE_API_UI_CSS_URL
    unset DEFAULT_CATTLE_API_UI_INDEX
    unset DEFAULT_CATTLE_API_UI_JS_URL

    export HASH=none
    export CATTLE_IDEMPOTENT_CHECKS=false
    export CATTLE_RANCHER_COMPOSE_VERSION latest
    export DEFAULT_CATTLE_RANCHER_COMPOSE_LINUX_URL=https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-linux-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
    export DEFAULT_CATTLE_RANCHER_COMPOSE_DARWIN_URL=https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-darwin-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
    export DEFAULT_CATTLE_RANCHER_COMPOSE_WINDOWS_URL=https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-windows-386-${CATTLE_RANCHER_COMPOSE_VERSION}.zip
    export CATTLE_RANCHER_CLI_VERSION latest
    export DEFAULT_CATTLE_RANCHER_CLI_LINUX_URL=https://releases.rancher.com/cli/${CATTLE_RANCHER_CLI_VERSION}/rancher-linux-amd64-${CATTLE_RANCHER_CLI_VERSION}.tar.gz
    export DEFAULT_CATTLE_RANCHER_CLI_DARWIN_URL=https://releases.rancher.com/cli/${CATTLE_RANCHER_CLI_VERSION}/rancher-darwin-amd64-${CATTLE_RANCHER_CLI_VERSION}.tar.gz
    export DEFAULT_CATTLE_RANCHER_CLI_WINDOWS_URL=https://releases.rancher.com/cli/${CATTLE_RANCHER_CLI_VERSION}/rancher-windows-386-${CATTLE_RANCHER_CLI_VERSION}.zip

    mkdir -p /source
    cd /source
    get_source

    cd cattle
    cattle-binary-pull ./modules/resources/src/main/resources/cattle-global.properties /usr/bin >/tmp/download.log 2>&1 &
    cd ..

    build_source

    cd cattle
    ./gradlew distZip
    EXTRACT=$(mktemp -d)
    ZIP=$(readlink -f modules/main/build/distributions/cattle*.zip)
    cd $EXTRACT
    unzip $ZIP
    SCRIPT=$(readlink -f */bin/cattle)
    run
}

get_source()
{
    if [[ ! -e cattle || -e .cattle.default ]] && ! echo "$REPOS" | grep -q cattle; then
        REPOS="$REPOS cattle"
        touch .cattle.default
    fi
    for r in $REPOS; do
        if ! [[ $r =~ ^http || $r =~ ^git ]]; then
            r="https://github.com/rancher/$r"
        fi
        tag=$(echo $r | cut -f2 -d,)
        r=$(echo $r | cut -f1 -d,)
        d=$(echo $r | awk -F/ '{print $NF}' | cut -f1 -d.)
        if [[ -z "$tag" || "$tag" = "$r" ]]; then
            tag=origin/master
        fi
        if [ -e $d ]; then
            git -C $d fetch origin
            git -C $d reset --hard $tag
        else
            git clone $r $d
            git -C $d checkout --detach $tag
        fi
    done
}

build_source()
{
    for i in *; do
        if [[ ! -d $i || $i == cattle ]]; then
            continue
        fi

        if [ ! -x "$(which make)" ]; then
            apt-get update
            apt-get install -y make
        fi

        if [ ! -x "$(which docker)" ]; then
            curl -sLf https://get.docker.com/builds/Linux/x86_64/docker-1.10.3 > /usr/bin/docker
            chmod +x /usr/bin/docker
        fi

        cd $i
        make build 2>&1 | xargs -I{} echo $i '|' "{}"
        ln -sf $(pwd)/bin/* /usr/local/bin/
        if [ "$i" = "agent" ]; then
            export CATTLE_AGENT_PACKAGE_PYTHON_AGENT_URL=$(pwd)
        fi
        cd ..
    done
}

update-rancher-ssl

if [ "$CATTLE_MASTER" = true ]; then
    master
else
    run
fi
