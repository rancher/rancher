---
title: Adding Hosts 
layout: default
---

## Adding Hosts
---

Within Rancher, we provide easy instructions to add your host from the Cloud providers that are supported directly from our UI as well as instructions to add your own host if your Cloud provider is not supported yet. From the **Hosts** tab within the Infrastructure tab, you click on **Add Host**.

### Hosts Requirements

* Docker 1.6.2+ ([Steps]({{site.baseurl}}/docs/installing-rancher/installing-server/#docker-install) on how to update to the latest Docker binary)
* Ubuntu 14.04, CoreOS 494+, CentOS 6/7, RHEL 6/7 
* Recommended CPU w/ AES-NI 
* Note: These are the only tested distributions at the moment, but most modern Linux distributions will work.

### Host Registration

The first time that you add a host, you may be required to set up the [Host Registration]({{site.baseurl}}/docs/configuration/host-registration/). This setup determines what DNS name or IP address, and port that your hosts will be connected to the Rancher API. By default, we have selected the management server IP and port `8080`.  If you choose to change the address, please make sure to specify the port that should be used to connect to the Rancher API. At any time, you can update the [Host Registration]({{site.baseurl}}/docs/configuration/host-registration/). After setting up your host registration, click on **Save**.

### Hosts

We support adding hosts directly from cloud providers or adding a host that's already been provisioned. Select which host type you want to add:

* [Adding DigitalOcean Droplet Hosts]({{site.baseurl}}/docs/infrastructure/hosts/digitalocean/)
* [Adding Amazon EC2 Hosts]({{site.baseurl}}/docs/infrastructure/hosts/amazon/)
* [Adding Packet Hosts]({{site.baseurl}}/docs/infrastructure/hosts/packet/)
* [Adding Rackspace Hosts]({{site.baseurl}}/docs/infrastructure/hosts/rackspace/)
* [Adding Custom Hosts]({{site.baseurl}}/docs/infrastructure/hosts/custom/)

When a host is added to Rancher, an agent container is launched on the host. Rancher will automatically pull the correct image version tag for rancher/agent image and run the required version. The agent version is tagged specifically to each Rancher server version.

<a id="hostlabels"></a>
### Host Labels

With each host, you have the ability to add labels to help you organize your hosts. The labels are a key/value pair and the keys must be unique identifiers. If you added two keys with different values, we'll take the last inputted value to use as the key/value pair.

By adding labels to hosts, you can use these labels when [scheduling services]({{site.baseurl}}/docs/services/projects/adding-services/#scheduling-services) and create a whitelist or blacklist of hosts for your [services]({{site.baseurl}}/docs/services) to run on. 

<a id="machine-config"></a>
### Host Access for Hosts created by Rancher

After Rancher launches the host, you may want to be able to access the host. We provide all the certificates generated when launching the machine in an easy to download file. Click on **Machine Config** in the host's dropdown menu. It will download a tar.gz file that has all the certificates.

To SSH into your host, go to your terminal/command prompt. Navigate to the folder of all the certificates and ssh in using the `id_rsa` certificate.

```bash
$ ssh -i id_rsa root@<IP_OF_HOST>
```

### Cloning a Host

Since launching hosts on cloud providers requires using an access key, you might want to easily create another host without needing to input all the credentials again. Rancher provides the ability to clone these credentials to spin up a new host. Select **Clone** from the host's drop down menu. It will bring up an **Add Host** page with the credentials of the cloned host populated.



