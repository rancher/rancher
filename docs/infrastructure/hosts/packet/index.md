---
title: Packet Hosts 
layout: default
---

## Adding Packet Hosts
---

Rancher supports provisioning [Packet](https://www.packet.net/) hosts using `docker machine`. 

### Finding Packet Credentials

In order to launch a Packet host, you'll need an **API Key**. Log in to your Packet account.

1. Navigate to the [api-key page](https://app.packet.net/portal#/api-keys). If you haven't created an api key, you'll need to add one.

    ![Packet Hosts on Rancher 1]({{site.baseurl}}/img/rancher_packet_1.png)

2. In the new api key screen, you'll put in a description (e.g. Rancher) and click **Generate**.

    ![Packet Hosts on Rancher 2]({{site.baseurl}}/img/rancher_packet_2.png)

3. The newly created **Token** will be visible for you to copy and use in Rancher. 

    ![Packet Hosts on Rancher 3]({{site.baseurl}}/img/rancher_packet_3.png)

### Launching Packet Host(s)

Now that we've found our **Token**, we are ready to launch our Packet host(s). Under the **Infrastructure -> Hosts** tab, click **Add Host**. Select the **Packet** icon. 

1. Select the number of hosts you want to launch using the slider.
2. Provide a **Name** and if desired, **Description** for the host.
3. Provide the **API Key** that you have created from your Packet account.
4. Provide the **Project** that you want the host to be launched. This project is found in your Packet account. 
5. Select the **Image**. Whatever `docker machine` supports for Packet is also supported by Rancher.
5. Select the **Size** of the image. 
6. Select the **Region** that you want to launch in. 
7. (Optional) Add **[labels]({{site.baseurl}}/docs/infrastructure/hosts/#labels)** to hosts to help organize your hosts and to [schedule services]({{site.baseurl}}/docs/services/projects/adding-services/#scheduling-services).
8. When complete, click **Create**. 

Once you click on create, Rancher will create the Packet and launch the _rancher-agent_ container. In a minute or two, the host will be active and available for [services]({{site.baseurl}}/docs/services/projects/adding-services/).

