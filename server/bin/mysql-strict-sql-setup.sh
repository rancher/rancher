#!/bin/bash

set -e

add_db_change_log()
{
    local db_user=${CATTLE_DB_CATTLE_USERNAME}
    local db_pass=${CATTLE_DB_CATTLE_PASSWORD}
    local db_name=${CATTLE_DB_CATTLE_MYSQL_NAME}

    local host=${CATTLE_DB_CATTLE_MYSQL_HOST:-127.0.0.1}
    local port=${CATTLE_DB_CATTLE_MYSQL_PORT:-3306}

    mysql -u${db_user} -p${db_pass} -h${host} -P${port} ${db_name}<< EOF 
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

add_db_change_log
