---
title: HA
layout: default
---

## Installing Rancher Server (HA)
---

As of the Beta release, Rancher server is capable of running in a HA configuration. We recognize that the setup is complex, and we will be working on making it easier to stand up as we approach GA. In the meantime, if you would like to experiment with Rancher running in HA, here is the basic outline.

### Pre-requisites

To launch a HA configuration Rancher needs the following:

*  Shared MySQL DB instance
*  Redis
*  Zookeeper
*  Load balancer to spread traffic across the Rancher instances


Documentation for building and scaling reliable Redis and Zookeeper installations are outside the scope of this document. As far as Rancher is concerned though, Redis and Zookeeper do not need to persist the data used by Rancher. If either ZooKeeper or Redis go down, Rancher will also go down, but the data in those system does not need to be present to recover. 

For MySQL, you can run your own or use MySQL provided by a cloud provider. We have used Google Cloud SQL and AWS RDS MySQL. 

Load balancing configuration can be handled in a number of ways. In our example configuration, servers are used in a round-robin configuration. The most basic health check that could be used is hitting the `/ping` url. It does not require authentication to receive the `pong` response. 


### Configuration

When launching rancher server, the following environment variables will need to be set:

* Database:
  * CATTLE_DB_CATTLE_MYSQL_HOST: `hostname or IP of MySQL instance`
  * CATTLE_DB_CATTLE_MYSQL_PORT: `3306`
  * CATTLE_DB_CATTLE_MYSQL_NAME: `Name of Database`
  * CATTLE_DB_CATTLE_USERNAME: `Username`
  * CATTLE_DB_CATTLE_PASSWORD: `Password`
* Zookeeper:    
  * CATTLE_ZOOKEEPER_CONNECTION_STRING: `comma separated list of zookeeper IPs ie. 10.0.1.2,10.0.1.3 will try connecting to 2181. Add :<port> for non-standard ports `
* Redis:
  * CATTLE_REDIS_HOSTS: `comma separated list of host:port server ips. ie 10.0.1.3:6379,10.0.1.4:6379`
  * CATTLE_REDIS_PASSWORD: `optional Redis password`

### Steps

1. Each server must have the basic [system requirements]({{site.baseurl}}/docs/installing-rancher/installing-server/) needed to run Rancher
2. Verify all servers can talk to your Redis installation
3. Verify all servers can talk to your ZooKeeper installation
4. Setup your MySQL database: You will need to create a database and user before starting Rancher server.

    ```sql
    CREATE DATABASE IF NOT EXISTS cattle COLLATE = 'utf8_general_ci' CHARACTER SET = 'utf8';
    GRANT ALL ON cattle.* TO 'cattle'@'%' IDENTIFIED BY 'cattle';
    GRANT ALL ON cattle.* TO 'cattle'@'localhost' IDENTIFIED BY 'cattle'; 
    ```

5. Launch your rancher server instances with this command:
      
      ```bash
      sudo docker run -d --restart=always -p 8080:8080 \
        -e CATTLE_DB_CATTLE_MYSQL_HOST: <hostname or IP of MySQL instance> \
        -e CATTLE_DB_CATTLE_MYSQL_PORT: <port> \
        -e CATTLE_DB_CATTLE_MYSQL_NAME: <Name of Database> \
        -e CATTLE_DB_CATTLE_USERNAME: <Username> \
        -e CATTLE_DB_CATTLE_PASSWORD: <Password> \
        -e CATTLE_ZOOKEEPER_CONNECTION_STRING: <comma separated list of zookeeper IPs ie. 10.0.1.2,10.0.1.3> \
        -e CATTLE_REDIS_HOSTS: <comma separated list of host:port server ips. ie 10.0.1.3:6379,10.0.1.4:6379> \
        -e CATTLE_REDIS_PASSWORD: <optional Redis password> \
        rancher/server
      ```  
      
6. Point your load balancer at the server targets
7. Go to new installation ip: `http://<LB ip>:<port>` 
