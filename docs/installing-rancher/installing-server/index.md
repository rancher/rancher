---
title: Installing Rancher Server
layout: default
---

## Installing Rancher Server
---
Rancher is deployed as a set of Docker containers. Running Rancher is a simple as launching two containers. One container as the management server and another container on a node as an agent. 

### Requirements

* Docker 1.6.2+ ([Steps]({{site.baseurl}}/docs/installing-rancher/installing-server/#docker-install) on how to update to the latest Docker binary)
* Ubuntu 14.04, CoreOS 494+, CentOS 6/7, RHEL 6/7 
* 1GB RAM 
* Note: These are the only tested distributions at the moment, but most modern Linux distributions will work.

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
<a id="docker-install"></a>

## Updating to Latest Docker Binary

### Ubuntu 14.04

Please refer to the official Docker [documentation](https://docs.docker.com/installation/ubuntulinux/) for how to install Docker on Ubuntu 14.04.

```bash
# Get the latest Docker version
$ sudo wget -qO- https://get.docker.com/ | sh
# Check the Docker version
$ sudo docker version
```

### CentOS/RHEL 6/7

Please refer to the official Docker documentation for how to install Docker on [CentOS](https://docs.docker.com/installation/centos/) or [RHEL](https://docs.docker.com/installation/rhel/).

**CentOS/RHEL 7**

```bash
# Stop the Docker daemon
$ sudo systemctl stop docker.service
# Install wget for CentOS 7
$ sudo yum install wget
# Get the latest Docker version 
$ sudo wget https://get.docker.com/builds/Linux/x86_64/docker-latest -O /usr/bin/docker
# Start the Docker daemon
$ sudo systemctl start docker.service
# Check the Docker version
$ sudo docker version
```

RHEL 7/Docker 1.6.2: The service will fail to start due to an invalid argument. We need to remove one of the arguments in the Docker service file and reboot. 

```bash
# Edit the service file if docker fails to start
$ sudo vi /usr/lib/systemd/system/docker.service
```
Remove the `$ADD_REGISTRY` argument from the file, reboot and attempt to start the Docker service again.

```bash
# Start the Docker daemon
$ sudo service docker start
# Check the Docker version
$ sudo docker version
```

**CentOS/RHEL 6**

```bash
# Stop the Docker daemon
$ sudo service docker stop
# Get the latest Docker version 
$ sudo wget https://get.docker.com/builds/Linux/x86_64/docker-latest -O /usr/bin/docker
# Start the Docker daemon
$ sudo service docker start
# Check the Docker version
$ sudo docker version
```
