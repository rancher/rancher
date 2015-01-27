#Rancher.io

Rancher is an open source project that provides infrastructure services designed specifically for Docker. Rancher makes AWS-like functions, such as EBS, VPC, ELB and Security Groups, available and consistent across any servers running locally or in any cloud.

## Why?

Implicitly most applications take advantage of storage and networking infrastructure services in one way or another.  If you are running on AWS today and you are using any functionality from EBS, VPC, etc. you are effectively limiting the portability of your application.  While Docker provides a portable compute unit, Rancher.io aims to complete the picture by providing portable implementations of storage and networking services.

## Installation

Rancher is deployed as a set of Docker containers.  Running Rancher is a simple as launching two containers.  One container as the management server and another container on a node as an agent.  You can install the containers in following approaches.

* [Manually](#installation)
* [Vagrant](#vagrant)
* [Puppet](https://github.com/nickschuch/puppet-rancher) (Thanks @nickschuch) 

### Requirements

* Docker 1.3+
* Ubuntu 14.04 or CoreOS 494+
    * *Note: These are the only tested distributions at the moment, but most modern Linux distributions will work*

### Management Server

    docker run -d -p 8080:8080 rancher/server

### Register Docker Nodes

    docker run --rm -it --privileged -v /var/run/docker.sock:/var/run/docker.sock rancher/agent http://MANAGEMENT_IP:8080

The UI/API is available on the exposed port 8080.

## UI

The UI is available by accessing the base HTTP URL of the management server.  For example, http://server:8080

![UI](docs/host.png)

## API

The API is available by accessing the `/v1` HTTP path of the management server.  For example, http://server:8080/v1

Rancher has its own API for infrastructure management tasks.  For Docker related operations, the intention is to support the Docker CLI.  That work is currently in progress.

### Vagrant

If you want to use Vagrant to run this on your laptop just clone the repo and to do `vagrant up` and then access port 8080 for the UI.

## Status

We've just recently kicked off this project.  Currently Rancher.io is able to provide a basic implementation of overlay networking and cross-server Docker links.  A lot of work has been done to put in a solid orchestration platform to control all the functionality we wish to do.  Now that that framework is in place expect this project to produce a high amount of features over the next six months.

## Source Code

This repo is a meta-repo used for packaging.  The source code for Rancher is in other repos in the rancherio organization.  The majority of the code is in https://github.com/rancherio/cattle.

## Planned

* Storage
    * Docker volume management (create, delete, list)
    * Volume snapshot
    * Snapshot backup to S3/Object Store
    * Create volume from snapshot
* Networking
    * Security groups
    * Load balancing

