---
title: Hosts 
layout: default
---

## Getting Started with Hosts
---
Within Rancher, we provide easy instructions to add your host from the Cloud providers that are supported directly from our UI as well as instructions to add your own host if your Cloud provider is not supported yet.

<a id="addhost"></a>
## Adding a Host
---

From the **Hosts** tab within the Infrastructure tab, you click on **Add Host**.

The first time that you add a host, you may be required to set up the [Host Registration]({{site.baseurl}}/docs/configuration/host-registration/). This setup determines what DNS name or IP address, and port that your hosts will be connected to the Rancher API. By default, we have selected the management server IP and port `8080`.  If you choose to change the address, please make sure to specify the port that should be used to connect to the Rancher API. At any time, you can update the [Host Registration]({{site.baseurl}}/docs/configuration/host-registration/). After setting up your host registration, click on **Save**.

We support adding hosts directly from cloud providers or adding a host that's already been provisioned. Select which host type you want to add:

* [Adding DigitalOcean Droplet Hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/digitalocean/)
* [Adding Amazon EC2 Hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/amazon/)
* [Adding Packet Hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/packet/)
* [Adding Rackspace Hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/rackspace/)
* [Adding Custom Hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/custom/)

When a host is added to Rancher, an agent container is launched on the host. Rancher will automatically pull the correct image version tag for the `rancher/agent` and run the required version. The agent version is tagged specifically to each Rancher server version.

<a id="labels"></a>
### Host Labels

With each host, you have the ability to add labels to help you organize your hosts. The labels are a key/value pair and the keys must be unique identifiers. If you added two keys with different values, we'll take the last inputted value to use as the key/value pair.

By adding labels to hosts, you can use these labels when [scheduling services]({{site.baseurl}}/docs/rancher-ui/applications/stacks/adding-services/#scheduling-services) and create a whitelist or blacklist of hosts for your [services]({{site.baseurl}}/docs/rancher-ui/applications/stacks/adding-services/) to run on. 

<a id="machine-config"></a>
### Host Access for Hosts created by Rancher

After Rancher launches the host, you may want to be able to access the host. We provide all the certificates generated when launching the machine in an easy to download file. Click on **Machine Config** in the host's dropdown menu. It will download a tar.gz file that has all the certificates.

To SSH into your host, go to your terminal/command prompt. Navigate to the folder of all the certificates and ssh in using the `id_rsa` certificate.

```bash
$ ssh -i id_rsa root@<IP_OF_HOST>
```

### Cloning a Host

Since launching hosts on cloud providers requires using an access key, you might want to easily create another host without needing to input all the credentials again. Rancher provides the ability to clone these credentials to spin up a new host. Select **Clone** from the host's drop down menu. It will bring up an **Add Host** page with the credentials of the cloned host populated.

## Editing Hosts
---

The options for what can be done to a host are located in the host's dropdown. From the **Infrastructure** -> **Hosts** page, the dropdown icon will appear when you hover over the host. If you click on the host name to view more details of a host, the dropdown icon is located in the upper right corner of the page. It's located next to the State of the host.

If you select **Edit**, you can update the name, description or labels on the host. 

### Deactivating/Activating Hosts

Deactivating the host will put the host into an _Inactive_ state. In this state, no new containers can be deployed. Any active containers on the host will continue to be active and you will still have the ability to perform actions on these containers (start/stop/restart). The host will still be connected to the Rancher server. Select **Deactivate** from the host's dropdown menu.

When a host is in the _Inactive_ state, you can bring the host back into an _Active_ state by clicking on **Activate** from the host's dropdown menu.

### Removing Hosts

In order to remove a host from the server, you will need to do a couple of steps from the dropdown menu.

Select **Deactivate**. When the host has completed the deactivation, the host will display an _Inactive_ state. Select **Delete**. The server will start the removal process of the host from the Rancher server instance. The first state that it will display after it’s finished deleting it will be _Removed_. It will continue to finalize the removal process and move to a _Purged_ state before immediately disappearing from the UI. 

If the host was created on a cloud provider using Rancher, the host will be deleted from the cloud provider. If the host was added by using the [custom command]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/custom/), the host will remain on the cloud provider.
                                                                                                                                                                                                                                                                                                                                                        
> **Notes:** For custom hosts, all containers including the Rancher agent will continue to remain on the host.  

## Deleting Hosts outside of Rancher
---

If your host is deleted outside of Rancher, then Rancher server will continue to show the host until it’s removed. Eventually, these hosts will show up in a _Reconnecting_ state and never be able to reconnect. You will be able to **Delete** these hosts to remove them from the UI. 


