---
title: Installing Rancher Server
layout: default
---

## Installing Rancher Server
---
Rancher is deployed as a set of Docker containers. Running Rancher is a simple as launching two containers. One container as the management server and another container on a node as an agent. 

### Requirements

* Any modern Linux distribution that supports Docker 1.6+. (Ubuntu, RHEL/CentOS 6/7 are more heavily tested.) 
* 1GB RAM 

### Launching Rancher Server 

On your Linux machine with Docker installed, the command to start Rancher is simple.

```bash
sudo docker run -d --restart=always -p 8080:8080 rancher/server
```

The UI and API will be available on the exposed port `8080`. After the docker image is downloaded, it will take a minute or two before Rancher has successfully started. The IP of the machine will need to be public and accessible from the internet in order for Rancher to work.

You can access the UI by going to the following URL. The `server_ip` is the public IP address of the host that is running Rancher server.

`http://server_ip:8080`

Once the UI is up and running, you can start [adding hosts]({{site.baseurl}}/docs/infrastructure/hosts/). After the hosts are setup, you can start adding [services]({{site.baseurl}}/docs/services/projects/adding-services/).

<a id="external-db"></a>

### Using an external Database

If you require using an external database to run Rancher server, please follow these instructions to connect Rancher server to the database. Your database will already need to be created, but does not need any schemas created. Rancher will automatically create all the schemas related to Rancher.

The following environment variables will need to be passed within the `docker run` command in order to decouple the server from the DB. 

* CATTLE_DB_CATTLE_MYSQL_HOST: `hostname or IP of MySQL instance`
* CATTLE_DB_CATTLE_MYSQL_PORT: `3306`
* CATTLE_DB_CATTLE_MYSQL_NAME: `Name of Database`
* CATTLE_DB_CATTLE_USERNAME: `Username`
* CATTLE_DB_CATTLE_PASSWORD: `Password`


> **Note:** The name and user of the database must already exist in order for Rancher to be able to create the database schema. Rancher will not create the database. 

Here is the SQL command to create a database and users.

 ```sql
 CREATE DATABASE IF NOT EXISTS cattle COLLATE = 'utf8_general_ci' CHARACTER SET = 'utf8';
 GRANT ALL ON cattle.* TO 'cattle'@'%' IDENTIFIED BY 'cattle';
 GRANT ALL ON cattle.* TO 'cattle'@'localhost' IDENTIFIED BY 'cattle';
 ```

After the database and user is created, you can run the command to launch rancher server.

```bash
sudo docker run -d --restart=always -p 8080:8080 \
    -e CATTLE_DB_CATTLE_MYSQL_HOST: <hostname or IP of MySQL instance> \
    -e CATTLE_DB_CATTLE_MYSQL_PORT: <port> \
    -e CATTLE_DB_CATTLE_MYSQL_NAME: <Name of Database> \
    -e CATTLE_DB_CATTLE_USERNAME: <Username> \
    -e CATTLE_DB_CATTLE_PASSWORD: <Password> \
    rancher/server
```

<!--### Launching Rancher Server behind a proxy

In order to set up a HTTP proxy, you'll need to edit the Docker daemon to point to the proxy. Before attempting to start Rancher server, you'll need to edit the `/etc/default/docker` file to point to your proxy and restart Docker.

```bash
$ sudo vi /etc/default/docker
```

Within the file, edit the `#export http_proxy="http://127.0.0.1:3128/"` to have it point to your proxy. Save your changes and then restart docker. Restarting Docker is different on every OS. 

You'll need to add in environment variables in order for the Rancher server to use the proxy.

Potential Environment Variables to Set:
* http_proxy
* https_proxy
* no_proxy

```bash
$ sudo docker run -d -e http_proxy=<proxyURL> -e https_proxy=<proxyURL> -e NO_PROXY=<proxyURL> --restart=always -p 8080:8080 rancher/server
```
-->
