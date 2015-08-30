#!/bin/bash

set -e

DATADIR='/var/lib/mysql'

check_mysql_action()
{
    local action=$1

    local cmd1="break"
    local cmd2="sleep 1"
    if [ "${action}" == "stop" ]; then
        cmd1="sleep 1"
        cmd2="break"
    fi

    set +e
    for ((i=0;i<60;i++))
    do
        if mysqladmin status 2> /dev/null; then
            ${cmd1}
        else
            if [ "$i" -eq "59" ]; then
                echo "Could not ${action} MySQL..." 1>&2
                exit 1
            fi
            ${cmd2}
        fi
    done
    set -e
}

init_new_data_dir()
{
    local pidfile="${DATADIR}/mysql.pid"

    # If a blank directory is bind mounted, configure it.
    echo "Running mysql_install_db..."
    mysql_install_db --user=mysql --datadir="${DATADIR}" --rpm --basedir=/usr

    echo "Starting MySQL to initialize..."
    mysqld --user=mysql --datadir="${DATADIR}" --skip-networking --basedir=/usr --socket=/var/run/mysqld/mysqld.sock --pid-file="${pidfile}" &
    echo "Waiting for mysql to start"
    check_mysql_action start

    mysql_tzinfo_to_sql /usr/share/zoneinfo |mysql --protocol=socket -uroot mysql

    kill $(<"${pidfile}")
    check_mysql_action stop
    echo "Exiting MySQL initialization"
}


config_mysql()
{
    sed -i 's/^\(bind-address.*\)$/#\1/' /etc/mysql/my.cnf
    sed -i 's/^#\(max_connections.*\)/\1/;s/100$/1000/' /etc/mysql/my.cnf
    sed -i 's/^key_buffer[[:space:]]/key_buffer_size/' /etc/mysql/my.cnf

    # setup to be a master
    if [ "$(grep ^#server-id /etc/mysql/my.cnf)" ]; then
        sed -i 's/^#\(server-id.*\)/\1/' /etc/mysql/my.cnf
        sed -i 's/^#\(log_bin.*\)/\1/' /etc/mysql/my.cnf
        sed -i '/^log_bin.*$/a innodb_flush_log_at_trx_commit = 1' /etc/mysql/my.cnf
        sed -i '/^log_bin.*$/a sync_binlog           = 1' /etc/mysql/my.cnf
    fi
}


start_mysql()
{
    s6-svc -u ${S6_SERVICE_DIR}/mysql
    check_mysql_action start
}


setup_cattle_db()
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

## Boot2docker hack
if [ "$(lsmod | grep vboxguest)" ]; then
    echo "Running in VBox change mysql UID"
    uid=$(stat -c "%u" ${DATADIR})
    usermod -u ${uid} mysql
    chown -R mysql /var/run/mysqld
    chown -R mysql /var/log/mysql
fi

if [ ! -d "${DATADIR}/mysql" ]; then
    init_new_data_dir
fi

config_mysql
start_mysql
setup_cattle_db
