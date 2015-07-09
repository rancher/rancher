---
title: Custom Hosts 
layout: default
---

## Adding Custom Hosts
---

If you already have Linux machines deployed and just want to add them into Rancher, click on the **Custom** icon. In the UI, you will be provided a docker command to run on any host. The `docker` command launches the _rancher-agent_ container on the host. 

If you are using different [projects]({{site.baseurl}}/docs/configuration/projects/), the command provided through the UI will be unique to whatever **project** that you are in.

Please make sure that you are in the project that you want to add hosts to. The project is displayed in the upper right corner next to the account dropdown. When you first login to the Rancher instance, you are in the **Default** project.

For any hosts that are added, please make sure that any security groups or firewalls allow traffic. If these are not enabled, then the functionality of Rancher will be limited.

* From and To all other hosts on UDP ports `500` and `4500` (for IPsec networking)

As of our Beta release (v0.24.0), we no longer require any additional TCP ports. But if you are using a version prior to Beta, then you will need to add the following ports:

* From the internet to TCP ports `9345` and `9346` (for UI hosts stats/graphs) 

Once your hosts are added to Rancher, they are available for [our services]({{site.baseurl}}/docs/rancher-ui/applications/stacks/adding-services/).

<a id="samehost"></a>
### Adding Hosts to the same machine as Rancher Server

If you are adding an agent host on the same machine as Rancher server, you will need to edit the command provided from the UI. In order for the _rancher-agent_ container to be launched correctly, you will need to set the `CATTLE_AGENT_IP` environment variable to the public IP of the VM that Rancher server is running on.

```bash
sudo docker run -d -e CATTLE_AGENT_IP=<IP_OF_RANCHER_SERVER> -v /var/run/docker....
```

If you have added a host onto the same host as Rancher server, please note that you will not be able to create any containers on the host that binds to port `8080`. Since the UI of the Rancher server relies on the `8080` port, there will be a port conflict and Rancher will stop working. If you require using port `8080` for your containers, you could launch Rancher server using a different port. 

### VMs with Private and Public IP Addresses

By default, the IP of a VM with a private IP and public IP will be set to the public IP. If you wanted to change the IP address to the private one, you'll need to edit the command provided from the UI. In order for the _rancher-agent_  container to be launched correctly, you will need to set the `CATTLE_AGENT_IP` environment variable to the private IP address.

```bash
sudo docker run -d -e CATTLE_AGENT_IP=<PRIVATE_IP> -v /var/run/docker....
```

> **Note**: When setting the private IP address, any existing containers in Rancher will not be part of the same managed network. 

### Hosts behind a Proxy

In order to set up a HTTP proxy, you'll need to edit the Docker daemon to point to the proxy. Before attempting to add the custom host, you'll need to edit the `/etc/default/docker` file to point to your proxy and restart Docker.

```bash
$ sudo vi /etc/default/docker
```

Within the file, edit the `#export http_proxy="http://127.0.0.1:3128/"` to have it point to your proxy. Save your changes and then restart docker. Restarting Docker is different on every OS. 

You'll need to add in environment variables in order for the Rancher agent to use the proxy.

Potential Environment Variables to Set:
* http_proxy
* https_proxy
* NO_PROXY (must be capitalized)

```bash
$ sudo docker run -d -e http_proxy=<proxyURL> -e https_proxy=<proxyURL> -e NO_PROXY=<proxyURL> -v /var/run/docker....
```
