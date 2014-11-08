#Rancher.io

Rancher is an open source project that provides infrastructure services designed specifically for Docker. Rancher makes AWS-like functions, such as EBS, VPC, ELB and Security Groups, available and consistent across any servers running locally or in any cloud.

## Why?

Implicitly most applications take advantage of storage and networking infrastructure services in one way or another.  If you are running on AWS today and you are using any functionality from EBS, VPC, etc. you are effectively tying your application to AWS.  While Docker provides a portable compute unit, Rancher.io aims to complete the picture by providing portable implementations of storage and networking.

## Installation

Start the management server

    docker run -d -p 8080:8080 rancher/server

Register a node by pointing it to the created management server

    docker run -v /var/run/docker.sock:/var/run/docker.sock rancher/agent http://MANAGE_IP:8080

### Vagrant

Just run `vagrant up` and then access port 8080 for the UI.

## UI

The UI is available by accessing the base HTTP URL of the management server.  For example, http://server:8080

## API

The API is available by accessing the `/v1` HTTP path of the management server.  For example, http://server:8080/v1

## Status

We've just recently kicked off this project.  Currently Rancher.io is able to provide a basic implementation of overlay networking and cross server Docker links.  A lot of work has been done to put in a solid orchestration platform to control all the functionality we wish to do.  Now that that framework is in place expect this project to produce a high amount of features over the next six months.

## Planned

* Storage
    * Docker volume management (create, delete, list)
    * Volume snapshot
    * Snapshot backup to S3/Object Store
    * Create volume from snapshot
* Networking
    * Security groups
    * Load balancing

