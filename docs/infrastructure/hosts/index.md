---
title: Hosts 
layout: default
---

## Getting Started with Hosts
---

### What is a Host?

A host is a Linux machine that is used to deploy and run all your applications. The machine can be physical or virtual, which allows you flexibility of how you want to set up your Rancher instance. 

Within Rancher, we provide easy instructions to add your host from the Cloud providers that are supported directly from our UI as well as instructions to add your own host if your Cloud provider is not supported yet.

### Adding a Host

There are two ways to easily add a host. Click on the **Hosts** icon in the sidebar.

**IMAGE NEEDED TO SHOW HOST ICON**
![Hosts on Rancher 1]({{site.baseurl}}/img/Rancher_hosts1.png)

* Option 1: Next to **Hosts**, click on the **+** to add a host. 
* Option 2: Click on the **+ Add Host** image that will is displayed after the last host on the page.  

**IMAGE NEEDED TO SHOW ADD HOST BUTTONS**
![Hosts on Rancher 2]({{site.baseurl}}/img/Rancher_hosts2.png)

<!--HIDE FOR SAAS Docs The first time that you add a host, you will be required to set up the [Host Registration]({{site.baseurl}}/docs/rancher-settings/host-registration/). This setup determines what DNS name or IP address, and port that your hosts will be connected to the Rancher API. By default, we have selected the management server IP and port `8080`.  If you choose to change the address, please make sure to specify the port that should be used to connect to the Rancher API. At any time, you can update the [Host Registration]({{site.baseurl}}/docs/rancher-settings/host-registration/).

**IMAGE NEEDED FOR HOST REGISTRATION**
![Hosts on Rancher 3]({{site.baseurl}}/img/Rancher_hosts-registration1.png)-->

We support adding hosts directly from cloud providers or adding existing hosts. An existing host is any Linux machine that is already provisioned. Select which host type you want to add:

* [Adding Existing Hosts]({{site.baseurl}}/docs/getting-started/hosts/custom/)
* [Adding Amazon EC2 Hosts]({{site.baseurl}}/docs/getting-started/hosts/amazon/)
* [Adding DigitalOcean Droplet Hosts]({{site.baseurl}}/docs/getting-started/hosts/digitalocean/)





