---
title: Access Control for Rancher
layout: default
---

## Access Control
---

### What is Access Control?

<span class="highlight">Provide a description of Access Control</span>

### Enabling Access Control

By default, access control is not configured. This means anyone who has the IP address of your Rancher instance will be able to use it and access the API. Your Rancher instance is open to the public! We recommend configuring Access Control after launching Rancher. In the account dropdown menu, click on **Access Control** within the Administation section or click on the **Settings** link in the red banner.

<img src="{{site.baseurl}}/img/rancher_access_control_1.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<img src="{{site.baseurl}}/img/rancher_access_control_2.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<p style="clear: both;">

Follow the directions to register Rancher as a GitHub application. After authenticating your Rancher instance with GitHub, access control will be enabled. With access control enabled, you will be able to manage different [environments]({{site.baseurl}}/docs/configuration/environments/) and share them with different groups of people.

### Site Access

By default, anyone who is a member or owner of an environment will have access to the Rancher instance. Alternatively, you can also invite members to access the Rancher instance, but when they log in, they will have their own default environment.


