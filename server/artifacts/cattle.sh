#!/bin/bash
set -e

cd /var/lib/cattle

JAR=/usr/share/cattle/cattle.jar
DEBUG_JAR=/var/lib/cattle/lib/cattle-debug.jar
LOG_DIR=/var/lib/cattle/logs
export S6_SERVICE_DIR=${S6_SERVICE_DIR:-$S6_SERVICE_DIR}

if [ "$URL" != "" ]
then
    echo Downloading $URL
    curl -s $URL > cattle-download.jar
    JAR=cattle-download.jar
fi

if [ -e $DEBUG_JAR ]; then
    JAR=$DEBUG_JAR
fi

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

start_local_mysql()
{
    sed -i 's/^\(bind-address.*\)$/#\1/' /etc/mysql/my.cnf
    s6-svc -u ${S6_SERVICE_DIR}/mysql

    set +e
    for ((i=0;i<60;i++))
    do
        if mysqladmin status 2> /dev/null; then
            break
        else
            if [ "$i" -eq "59" ]; then
                echo "Could not start MySQL..." 1>&2
                exit 1
            fi
                sleep 1
            fi
    done
    set -e
}

setup_local_db()
{
    local db_user=$CATTLE_DB_CATTLE_USERNAME
    local db_pass=$CATTLE_DB_CATTLE_PASSWORD
    local db_name=$CATTLE_DB_CATTLE_MYSQL_NAME

    echo "Setting up database"
    mysql -uroot<< EOF
CREATE DATABASE IF NOT EXISTS ${db_name} COLLATE = 'utf8_general_ci' CHARACTER SET = 'utf8';
GRANT ALL ON ${db_name}.* TO "${db_user}"@'%' IDENTIFIED BY "${db_pass}";
GRANT ALL ON ${db_name}.* TO "${db_user}"@'localhost' IDENTIFIED BY "${db_pass}";
EOF
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
            start_local_mysql
            setup_local_db
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
}

setup_graphite
setup_gelf
setup_mysql
setup_redis
setup_zk

env | grep CATTLE | grep -v PASS | sort

exec java ${CATTLE_JAVA_OPTS:--Xmx256m -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=$LOG_DIR} -jar $JAR "$@" $ARGS
