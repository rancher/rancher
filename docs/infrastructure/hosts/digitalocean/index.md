---
title: DigitalOcean Hosts 
layout: default
---

## Adding DigitalOcean Hosts
---

You can launch DigitalOcean hosts directly from the UI. If you don't have a DigitalOcean account, here's a [link](NEED LINK) to get free $10 credit. You'll just need to make sure you input billing information in order to activate the credit.

### Finding DigitalOcean information

In order to launch a DigitalOcean host, you'll need a **Personal Access Token** provided by Digital Ocean. Log in to your Digital Ocean account. 

1. Navigate to the [Apps & API page](https://cloud.digitalocean.com/settings/applications). 

    ![DO Hosts on Rancher 1]({{site.baseurl}}/img/Rancher_do1.png)

2. In the **Personal Access Tokens**, click on the **Generate New Token** button. Name your token (e.g. Rancher) and click **Generate Token**.

    ![DO Hosts on Rancher 2]({{site.baseurl}}/img/Rancher_do2.png)

3. Copy your **Access Token** from the UI and save it somewhere safe. This is the only time you will be able to see the access token. Next time you go to the page, the token will no longer be shown and you will not be able to retrieve it.


### Launching the DO Droplet

Now that we've saved the **Access Token**, we just need to complete the DigitalOcean host page.

**IMAGE NEEDED TO SHOW DO AddHost page**
![DO Hosts on Rancher 3]({{site.baseurl}}/img/Rancher_do3.png)

1. Provide a **Name** and if desired, **Description** for the host.
2. Fill in the **Access Token** that you have created for your DigitalOcean account.
3. Select the **Image** that you want launched.
4. Select the **Size** of the image. 
5. Pick the **Region** that you want to launch in. We've provided the available regions that can be launched using metadata. Some regions may not be included as the API doesn't support it.
6. If you want to enable any of the additional options, select those options.
7. When complete, click **Create**. 

Once you click on create, Rancher will create the DigitalOcean droplet and launch the _rancher-agent_ container in the droplet. In a minute or two, the host will be active and available to launch services. **ADD LINK TO LAUNCHING SERVICES**
