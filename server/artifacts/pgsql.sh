#!/bin/bash

set -e

DATADIR='/var/lib/pgsql/9.6'

init_new_data_dir()
{
    chown postgres:postgres /var/lib/pgsql
    su postgres -c "pg_ctl init -D ${DATADIR}"
}

config_pgsql()
{
    local CONFIG=${DATADIR}/postgresql.conf
    sed -i 's/^#\(listen_addresses.*\)/\1/;s/localhost/*/' ${CONFIG}
    sed -i 's/^#\(max_connections.*\)/\1/;s/\<100\>/1000/' ${CONFIG}
    sed -i 's/\(shared_buffers.*\)/\1/;s/128/256/' ${CONFIG}
    sed -i 's/^#\(logging_collector.*\)/\1/;s/off/on/' ${CONFIG}
    sed -i 's/^#\(log_filename.*\)/\1/;s/postgresql-%Y-%m-%d_%H%M%S.log/postgresql-%a.log/' ${CONFIG}
    sed -i 's/^#\(log_truncate_on_rotation.*\)/\1/;s/off/on/' ${CONFIG}
    sed -i 's/^#\(log_rotation_size.*\)/\1/;s/10MB/0/' ${CONFIG}
}

start_pgsql()
{
    s6-svc -u ${S6_SERVICE_DIR}/pgsql

    set +e
    for ((i=0;i<60;i++))
    do
        if su postgres -c "pg_ctl status -s -D ${DATADIR}" &>> /dev/null; then
            break
        else
            if [ "$i" -eq "59" ]; then
                echo "Could not start database..." 1>&2
                exit 1
            fi
                sleep 1
            fi
    done
    set -e
}

setup_cattle_db()
{
    local db_user=$CATTLE_DB_CATTLE_USERNAME
    local db_pass=$CATTLE_DB_CATTLE_PASSWORD
    local db_name=$CATTLE_DB_CATTLE_POSTGRES_NAME

    echo "Setting up database"
    su postgres -c "psql ${db_name} -c ''" || \
        su postgres -c "psql << EOF
            CREATE ROLE ${db_user} WITH LOGIN PASSWORD '${db_pass}';
            CREATE DATABASE ${db_name} OWNER ${db_user};"
}

if [ ! -d "${DATADIR}" ]; then
    init_new_data_dir
fi

config_pgsql
start_pgsql
setup_cattle_db
