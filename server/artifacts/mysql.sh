#!/bin/bash

set -e

DATADIR='/var/lib/mysql'
PIDFILE="/${DATADIR}/mysqld.pid"

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
    # If a blank directory is bind mounted, configure it.
    echo "Running mysql_install_db..."
    mysql_install_db --user=mysql --rpm

    echo "Starting MySQL to initialize..."
    start_admin_mysql

    mysql_tzinfo_to_sql /usr/share/zoneinfo | mysql --protocol=socket -uroot mysql

    stop_mysql
    echo "Exiting MySQL initialization"
}


config_mysql()
{
    grep -l . /etc/mysql/*.cnf | xargs \
        sed -i '/socket/s/=.*/= \/var\/lib\/mysql\/mysqld.sock/'
    sed -i 's/^\(bind-address.*\)$/#\1/' /etc/mysql/my.cnf
    sed -i 's/^#\(max_connections.*\)/\1/;s/100$/1000/' /etc/mysql/my.cnf
    sed -i 's/^key_buffer[[:space:]]/key_buffer_size/' /etc/mysql/my.cnf
    sed -i 's/^\(expire_logs_days.*\)/\1/;s/10$/2/' /etc/mysql/my.cnf
    sed -i '/^max_connections.*$/a sql_mode = ONLY_FULL_GROUP_BY' /etc/mysql/my.cnf

    if [ ! "$(grep innodb_file_per_table /etc/mysql/my.cnf)" ]; then
        sed -i '/^# \* InnoDB.*$/a innodb_file_per_table = 1' /etc/mysql/my.cnf
    fi
}

start_admin_mysql()
{
    mysqld --user=mysql --skip-networking --pid-file="${PIDFILE}" &
    echo "Waiting for mysql to start"
    check_mysql_action start
}

stop_mysql()
{
    kill $(<"${PIDFILE}")
    check_mysql_action stop
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

    if ! echo 'show tables' | mysql -uroot $db_name | grep -iq account; then
        echo "Importing schema"
        mysql -uroot $db_name < /usr/share/cattle/mysql-dump.sql
    fi

}


upgrade_mysql()
{
    chown -R mysql:mysql /var/lib/mysql
    chown mysql:root /var/lib/mysql/mysql

    echo "Starting MySQL to upgrade..."
    start_admin_mysql

    mysql_upgrade

    stop_mysql
    echo "Exiting MySQL upgrade"
}

## Boot2docker hack
if [ "$(grep /var/lib/mysql /proc/mounts|cut -d' ' -f3)" = "vboxsf" ]; then
    echo "Running in VBox change mysql UID"
    uid=$(stat -c "%u" ${DATADIR})
    usermod -u ${uid} mysql
    chown -R mysql /var/run/mysqld
    chown -R mysql /var/log/mysql
fi

config_mysql

if [ ! -d "${DATADIR}/mysql" ]; then
    init_new_data_dir
fi

upgrade_mysql
start_mysql
setup_cattle_db
