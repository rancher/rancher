---
title: Running Rancher 
layout: default
---

## Running Rancher
---
Rancher is deployed as a set of Docker containers. Running Rancher is a simple as launching two containers. One container as the management server and another container on a node as an agent. 

### Requirements

* Docker 1.6.0+
* Ubuntu 14.04 or CoreOS 494+
* <span class="highlight">RAM/CPU?</span>
* Note: These are the only tested distributions at the moment, but most modern Linux distributions will work.

### Launching Rancher Server 

On your Linux machine with Docker installed, the command to start Rancher is simple.

`sudo docker run -d --restart=always -p 8080:8080 rancher/server`

The UI and API will be available on the exposed port `8080`. After the docker image is downloaded, it will take a minute or two before Rancher has successfully started. The IP of the machine will need to be public and accessible from the internet in order for Rancher to work.

You can access the UI by going to the base URL of the management server. 

`http://machine_ip:8080`

Once the UI is up and running, you can start [adding hosts]({{site.baseurl}}/docs/infrastructure/hosts/). After the hosts are setup, you can start running [services]({{site.baseurl}}/docs/services/).

