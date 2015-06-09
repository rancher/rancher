---
title: Running Rancher 
layout: default
---

## Running Rancher
---
Rancher is deployed as a set of Docker containers. Running Rancher is a simple as launching two containers. One container as the management server and another container on a node as an agent. 

### Requirements

* Docker 1.6.2+ ([Steps]({{site.baseurl}}/docs/running-rancher/#docker-install) on how to update to the latest Docker binary)
* Ubuntu 14.04, CoreOS 494+, CentOS 6/7, RHEL 6/7 <span class="highlight">What else are we going to support?</span>
* <span class="highlight">RAM/CPU?</span>
* Note: These are the only tested distributions at the moment, but most modern Linux distributions will work.

### Launching Rancher Server 

On your Linux machine with Docker installed, the command to start Rancher is simple.

```bash
sudo docker run -d --restart=always -p 8080:8080 rancher/server
```

The UI and API will be available on the exposed port `8080`. After the docker image is downloaded, it will take a minute or two before Rancher has successfully started. The IP of the machine will need to be public and accessible from the internet in order for Rancher to work.

You can access the UI by going to the following URL. The `machine_ip` is the public IP address of the host that is running Rancher server.

`http://machine_ip:8080`

Once the UI is up and running, you can start [adding hosts]({{site.baseurl}}/docs/infrastructure/hosts/). After the hosts are setup, you can start running [services]({{site.baseurl}}/docs/services/).

<a id="external-db"></a>

### Using an external Database

If you require using an external database to run Rancher server, please follow these instructions to connect Rancher server to the database. Your database will already need to be created, but does not need any schemas created. Rancher will automatically create all the schemas related to Rancher.

The following environment variables will need to be passed within the `docker run` command in order to decouple the server from the DB. 

```bash
CATTLE_DB_CATTLE_MYSQL_HOST
CATTLE_DB_CATTLE_MYSQL_PORT
CATTLE_DB_CATTLE_USERNAME
CATTLE_DB_CATTLE_PASSWORD
CATTLE_DB_CATTLE_MYSQL_NAME
```

> **Note:** The `CATTLE_DB_CATTLE_MYSQL_NAME` must already exist in order for Rancher to be able to create the database schema. Rancher will not create the database.

```bash
sudo docker run -d --restart=always -p 8080:8080 -e CATTLE_DB_CATTLE_MYSQL_HOST=<location_of_db> -e CATTLE_DB_CATTLE_MYSQL_PORT=<port_of_db> -e CATTLE_DB_CATTLE_USERNAME=<username_for_db> -e CATTLE_DB_CATTLE_PASSWORD=<password_for_user> -e CATTLE_DB_CATTLE_MYSQL_NAME=<name_of_existing_db>  rancher/server
```

### Running Rancher behind a Proxy

In order to set up a HTTP proxy, you'll need to edit the Docker daemon to point to the proxy. Before launching Rancher, you'll need to edit the `/etc/default/docker` file to point to your proxy and restart Docker.

```bash
$ sudo vi /etc/default/docker
```

Within the file, edit the `#export http_proxy="http://127.0.0.1:3128/"` to have it point to your proxy. Save your changes and then restart docker. Restarting Docker is different on every OS. 

<a id="docker-install"></a>
## Installing the Latest Docker Versions

Please refer to the official Docker [documentation](https://docs.docker.com/installation/) on how to install Docker. We have provided a quick guide on how to get Docker up with the latest binary on some of our supported OSes. All of the directions are assuming that Docker is already installed and you just need to upgrade to latest binary.

<span class="highlight">How to update CoreOS?</span>

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
