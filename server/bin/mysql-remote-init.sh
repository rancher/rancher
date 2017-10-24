#!/bin/bash

set -e

export MYSQL_HOST=${CATTLE_DB_CATTLE_MYSQL_HOST:-127.0.0.1}
export MYSQL_PORT=${CATTLE_DB_CATTLE_MYSQL_PORT:-3306}

setup_cattle_db()
{
    local db_admin_password=$DB_ROOT_PASSWORD

    local db_user=$CATTLE_DB_CATTLE_USERNAME
    local db_pass=$CATTLE_DB_CATTLE_PASSWORD
    local db_name=$CATTLE_DB_CATTLE_MYSQL_NAME

    echo "Setting up database"
    mysql -uroot -p${db_admin_password} -h${MYSQL_HOST} -P${MYSQL_PORT}<< EOF
CREATE DATABASE IF NOT EXISTS ${db_name} COLLATE = 'utf8_general_ci' CHARACTER SET = 'utf8';
GRANT ALL ON ${db_name}.* TO "${db_user}"@'%' IDENTIFIED BY "${db_pass}";
GRANT ALL ON ${db_name}.* TO "${db_user}"@'localhost' IDENTIFIED BY "${db_pass}";
EOF
}

add_db_change_log()
{
    local db_user=$CATTLE_DB_CATTLE_USERNAME
    local db_pass=$CATTLE_DB_CATTLE_PASSWORD
    local db_name=$CATTLE_DB_CATTLE_MYSQL_NAME

    mysql -u${db_user} -p${db_pass} -h${MYSQL_HOST} -P${MYSQL_PORT} ${db_name}<< EOF 
CREATE TABLE IF NOT EXISTS \`DATABASECHANGELOG\` (
  \`ID\` varchar(255) NOT NULL,
  \`AUTHOR\` varchar(255) NOT NULL,
  \`FILENAME\` varchar(255) NOT NULL,
  \`DATEEXECUTED\` datetime NOT NULL,
  \`ORDEREXECUTED\` int(11) NOT NULL,
  \`EXECTYPE\` varchar(10) NOT NULL,
  \`MD5SUM\` varchar(35) DEFAULT NULL,
  \`DESCRIPTION\` varchar(255) DEFAULT NULL,
  \`COMMENTS\` varchar(255) DEFAULT NULL,
  \`TAG\` varchar(255) DEFAULT NULL,
  \`LIQUIBASE\` varchar(20) DEFAULT NULL,
  \`PKID\` int NOT NULL AUTO_INCREMENT PRIMARY KEY
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
EOF
}

setup_cattle_db

while [ "$#" -gt 0 ]; do
  case $1 in
    --strict-enforce)
        shift 1
        add_db_change_log
        ;;
    *)
      break
      ;;
  esac
done
