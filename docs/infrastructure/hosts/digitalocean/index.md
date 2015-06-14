---
title: DigitalOcean Hosts 
layout: default
---

## Adding DigitalOcean Hosts
---

Rancher supports provisioning [DigitalOcean](https://www.digitalocean.com/) hosts using `docker machine`. If you don't have a DigitalOcean account, here's a <span class="highlight">"[link]</span> to get free $10 credit. You'll just need to make sure you input billing information in order to activate the credit.

### Finding DigitalOcean Credentials

In order to launch a DigitalOcean host, you'll need a **Personal Access Token** provided by DigitalOcean. Log in to your DigitalOcean account. 

1. Navigate to the [Apps & API page](https://cloud.digitalocean.com/settings/applications). 

    ![DO Host on Rancher 1]({{site.baseurl}}/img/rancher_do_1.png)

2. In the **Personal Access Tokens**, click on the **Generate New Token** button. Name your token (e.g. Rancher) and click **Generate Token**.

    ![DO Host on Rancher 2]({{site.baseurl}}/img/rancher_do_2.png)

3. Copy your **Access Token** from the UI and save it somewhere safe. This is the only time you will be able to see the access token. Next time you go to the page, the token will no longer be shown and you will not be able to retrieve it.

### Launching DigitalOcean Host(s)

Now that we've saved the **Access Token**, we are ready to launch our DigitalOcean host. Under the **Infrastructure -> Hosts** tab, click **Add Host**. Select the **DigitalOcean** icon. 

1. Select the number of hosts you want to launch using the slider.
2. Provide a **Name** and if desired, **Description** for the host.
3. Provide the **Access Token** that you have created for your DigitalOcean account.
4. Select the **Image** that you want launched. Whatever `docker machine` supports for DigitalOcean is also supported by Rancher.
5. Select the **Size** of the image. 
6. Select the **Region** that you want to launch in. We've provided the available regions that can be launched using metadata. Some regions may not be included as the API doesn't support it.
7. (Optional) If you want to enable any of the advanced options (i.e. backups, IPv6, private networking), select the ones that you want to include.
8. (Optional) Add **[labels]({{site.baseurl}}/docs/infrastructure/hosts/#labels)** to hosts to help organize your hosts and to [schedule services]({{site.baseurl}}/docs/services/projects/adding-services/#scheduling-services).
9. When complete, click **Create**. 

Once you click on create, Rancher will create the DigitalOcean droplet and launch the _rancher-agent_ container in the droplet. In a couple of minutes, the host will be active and available for [services]({{site.baseurl}}/docs/services/).
