---
title: Custom Hosts 
layout: default
---

## Adding Custom Hosts
---

If you already have Linux machines deployed and just want to add them into Rancher, click on the **Custom** icon. In the UI, you will be provided a docker command to run on any host. The `docker` command launches the _rancher-agent_ container on the host. 

If you are using different [environments]({{site.baseurl}}/docs/configuration/environments/), the command provided through the UI will be unique to whatever **environment** that you are in.

Please make sure that you are in the environment that you want to add hosts to. The environment is displayed in the upper right corner next to the account dropdown. When you first login to the Rancher instance, you are in the **Default** environment.

![Environments on Rancher 1]({{site.baseurl}}/img/rancher_environments_1.png)

For any hosts that are added, please make sure that any security groups or firewalls allow traffic. If these are not enabled, then the functionality of Rancher will be limited.

* From the internet to TCP ports `9345` and `9346` (for UI hosts stats/graphs)
* From and To all other hosts on UDP ports `500` and `4500` (for IPsec networking)

Once your hosts are added to Rancher, they are available for [our services]({{site.baseurl}}/docs/services/).

<a id="samehost"></a>
### Adding Hosts to the same machine as Rancher Server

If you are adding an agent host on the same machine as Rancher server, you will need to edit the command provided from the UI. In order for the _rancher-agent_ container to be launched correctly, you will need to set the `CATTLE_AGENT_IP` environment variable to the public IP of the VM that Rancher server is running on.

```bash
sudo docker run -d -e CATTLE_AGENT_IP=<IP_OF_RANCHER_SERVER> -v /var/run/docker....
```

If you have added a host onto the same host as Rancher server, please note that you will not be able to create any containers on the host that binds to port `8080`. Since the UI of the Rancher server relies on the `8080` port, there will be a port conflict and Rancher will stop working.

### VMs with Private and Public IP Addresses

By default, the IP of a VM with a private IP and public IP will be set to the public IP. If you wanted to change the IP address to the private one, you'll need to edit the command provided from the UI. In order for the _rancher-agent_  container to be launched correctly, you will need to set the `CATTLE_AGENT_IP` environment variable to the private IP address.

```bash
sudo docker run -d -e CATTLE_AGENT_IP=<PRIVATE_IP> -v /var/run/docker....
```

> **Note**: When setting the private IP address, any existing containers in Rancher will not be part of the same managed network. 



