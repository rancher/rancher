---
title: Rancher Documentation
layout: default

---

## Rancher
---
The smallest, easiest way to run Docker in production at scale.  Everything in RancherOS is a container managed by Docker.  This includes system services such as udev and rsyslog.  RancherOS includes only the bare minimum amount of software needed to run Docker.  This keeps the binary download of RancherOS to about 20MB.  Everything else can be pulled in dynamically through Docker.

### How this works



Everything in RancherOS is a Docker container.  We accomplish this by launching two instances of Docker.  One is what we call the system Docker which runs as PID 1.  System Docker then launches a container that runs the user Docker.  The user Docker is then the instance that gets primarily used to create containers.  We created this separation because it seemed logical and also it would really be bad if somebody did 
`docker rm -f $(docker ps -qa)` and deleted the entire OS.



![How it works]({{site.baseurl}}/img/rancheroshowitworks.png "How it works")



## Running RancherOS
---
To find out more about installing RancherOS, read more about it on our [Getting Started Guide]({{site.baseurl}}/docs/getting-started/).


## Latest Release
---
Please check our repository for the latest release in our [README](https://github.com/rancherio/os/blob/master/README.md). 


## Cloud
---
Currently we only have RancherOS available in EC2 and GCE (experimental) but more clouds will come based on demand. Follow the steps in our [Amazon]({{site.baseurl}}/docs/getting-started/amazon/) or [GCE]({{site.baseurl}}/docs/getting-started/gce/) guide to get started.
<br>
<br>