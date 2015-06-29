---
title: Rackspace Hosts 
layout: default
---

## Adding Rackspace Hosts
---

Rancher supports provisioning [Rackspace](http://www.rackspace.com/) hosts using `docker machine`. 

### Finding Rackspace Credentials

In order to launch a Rackspace host, you'll need your **API Key** provided by Rackspace. Log in to your Rackspace account. 

1. Navigate to the Account Settings. 

2. In the Login Details section, there is an **API Key**. Click on Show to reveal the API Key. Copy the key to use in Rancher. 

### Launching Rackspace Host(s)

Now that we've found our **API Key**, we are ready to launch our Rackspace host(s). Under the **Infrastructure -> Hosts** tab, click **Add Host**. Select the **Rackspace** icon. 


1. Select the number of hosts you want to launch using the slider.
2. Provide a **Name** and if desired, **Description** for the host.
3. Provide the **Username** that you log in to your Rackspace account with.
4. Provide the **API Key** that we found associated with your username.
5. Pick the **Region** that you want to launch your host in.
6. Pick the **Flavor** of the host.
7. (Optional) Add **[labels]({{site.baseurl}}/docs/infrastructure/hosts/#labels)** to hosts to help organize your hosts and to [schedule services]({{site.baseurl}}/docs/services/projects/adding-services/#scheduling-services).
8. When complete, click **Create**. 

Once you click on create, Rancher will create the Rackspace server and launch the _rancher-agent_ container in the droplet. In a couple of minutes, the host will be active and available for [services]({{site.baseurl}}/docs/services/projects/adding-services/).
