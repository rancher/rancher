#!/bin/bash
set -e

cd /var/lib/cattle

JAR=/usr/share/cattle/cattle.jar
HASH=$(md5sum $JAR | awk '{print $1}')
DEBUG_JAR=/var/lib/cattle/lib/cattle-debug.jar
LOG_DIR=/var/lib/cattle/logs
export S6_SERVICE_DIR=${S6_SERVICE_DIR:-$S6_SERVICE_DIR}

if [ "$URL" != "" ]
then
    echo Downloading $URL
    curl -sLf $URL > cattle-download.jar
    JAR=cattle-download.jar
fi

if [ -e $DEBUG_JAR ]; then
    JAR=$DEBUG_JAR
fi

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

setup_redis()
{
    local hosts=""
    local i=1

    while [ -n "$(eval echo \$REDIS${i}_PORT_6379_TCP_ADDR)" ]; do
        local host="$(eval echo \$REDIS${i}_PORT_6379_TCP_ADDR:\$REDIS${i}_PORT_6379_TCP_PORT)"

        if [ -n "$hosts" ]; then
            hosts="$hosts,$host"
        else
            hosts="$host"
        fi

        i=$((i+1))
    done

    if [ -n "$hosts" ]; then
        export CATTLE_REDIS_HOSTS=${CATTLE_REDIS_HOSTS:-$hosts}
    fi

    if [ -n "$CATTLE_REDIS_HOSTS" ]; then
        export CATTLE_MODULE_PROFILE_REDIS=true
    fi
}

setup_zk()
{
    local hosts=""
    local i=1

    while [ -n "$(eval echo \$ZK${i}_PORT_2181_TCP_ADDR)" ]; do
        local host="$(eval echo \$ZK${i}_PORT_2181_TCP_ADDR:\$ZK${i}_PORT_2181_TCP_PORT)"

        if [ -n "$hosts" ]; then
            hosts="$hosts,$host"
        else
            hosts="$host"
        fi

        i=$((i+1))
    done

    if [ -n "$hosts" ]; then
        export CATTLE_ZOOKEEPER_CONNECTION_STRING=${CATTLE_ZOOKEEPER_CONNECTION_STRING:-$hosts}
    fi

    if [ -n "$CATTLE_ZOOKEEPER_CONNECTION_STRING" ]; then
        export CATTLE_MODULE_PROFILE_ZOOKEEPER=true
    fi

    if [ -n "$CATTLE_ZOOKEEPER_CONNECTION_STRING" ]; then
        local ok=false
        for ((i=0; i<=30; i++)); do
            local host="$(echo $CATTLE_ZOOKEEPER_CONNECTION_STRING | cut -f1 -d, | cut -f1 -d:)"
            local port="$(echo $CATTLE_ZOOKEEPER_CONNECTION_STRING | cut -f1 -d, | cut -f2 -d:)"
            echo Waiting for Zookeeper at ${host}:${port}
            if [ "$(echo ruok | nc $host $port)" == "imok" ]; then
                ok=true
                break
            fi
            sleep 2
        done
        if [ "$ok" != "true" ]; then
            echo Failed waiting for Zookeeper at ${host}:${port}
            return 1
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
    setup_gelf
    setup_mysql
    setup_redis
    setup_zk
    setup_proxy

    env | grep CATTLE | grep -v PASS | sort

    update-rancher-ssl

    local ram=$(free -g --si | awk '/^Mem:/{print $2}')
    if [ ${ram} -gt 6 ]; then
        MX="4g"
    elif [ ${ram} -gt 2 ]; then
        MX="2g"
    else
        MX="1g"
    fi

    HASH_PATH=$(dirname $JAR)/$HASH
    if [ -e $HASH_PATH ]; then
        if [ -e $HASH_PATH/index.html ]; then
            export DEFAULT_CATTLE_API_UI_INDEX=local
        fi
        exec java ${CATTLE_JAVA_OPTS:--XX:+UseConcMarkSweepGC -XX:+CMSClassUnloadingEnabled -Xms128m -Xmx${MX} -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=$LOG_DIR} -Dlogback.bootstrap.level=WARN $PROXY_ARGS $JAVA_OPTS -cp ${HASH_PATH}:${HASH_PATH}/etc/cattle io.cattle.platform.launcher.Main "$@" $ARGS
    else
        unset DEFAULT_CATTLE_API_UI_JS_URL
        unset DEFAULT_CATTLE_API_UI_CSS_URL
        exec java ${CATTLE_JAVA_OPTS:--XX:+UseConcMarkSweepGC -XX:+CMSClassUnloadingEnabled -Xms128m -Xmx${MX} -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=$LOG_DIR} $PROXY_ARGS $JAVA_OPTS -jar $JAR "$@" $ARGS
    fi
}

extract()
{
    cd $(dirname $JAR)
    java -jar $JAR war
    mkdir $HASH
    ln -s $HASH war
    cd war
    unzip ../*.war
    unzip $JAR etc\*
    rm ../*.war
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
    if [ ! -e cattle ]; then
        git clone https://github.com/rancher/cattle
        cd cattle
    elif [ ! -e cattle/.manual ]; then
        cd cattle
        git fetch origin
        git checkout master
        git reset --hard origin/master
        git clean -dxf
    else
        cd cattle
    fi

    cattle-binary-pull ./resources/content/cattle-global.properties /usr/bin >/tmp/download.log 2>&1 &

    if [ ! -x "$(which mvn)" ]; then
        apt-get update
        apt-get install --no-install-recommends -y maven openjdk-7-jdk
    fi

    mvn package
    wait || {
        cat /tmp/download.log
        exit 1
    }
    JAR=$(readlink -f code/packaging/app/target/cattle-app-*.war)
    run
}

if [ "$1" = "extract" ]; then
    extract
elif [ "$CATTLE_MASTER" = true ]; then
    master
else
    run
fi
