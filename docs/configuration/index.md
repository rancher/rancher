---
title: Rancher Configuration
layout: default
---

## Rancher Configuration
---

Configuring Rancher is very straightforward. All configuration options are with the account dropdown menu, which is located in the top right corner. 

When access control is turned on, the first user to authenticate with the instance will be set as the admin of the Rancher instance. They will be the only ones to have the ability to change the settings within **Administration**. Anyone else logging into the site will be given user rights and see only the **Settings** section.

### Administration

In [Access Control]({{site.baseurl}}/docs/configuration/access-control), you set up your instance to require authentication in order to access it. Configuring access control is highly recommended, as anyone with access to your Rancher IP would be able to use your Rancher instance and access the API. Once you configure access control, users will be required to use an API key in order to access the API.

In [Host Registration]({{site.baseurl}}/docs/configuration/host-registration), you set up how hosts should connect to the Rancher API.  

### Settings

In [API & Keys]({{site.baseurl}}/docs/configuration/api-keys/), you can view the link to the API as well as create API Keys to access the API. If your Rancher instance has Access Control enabled, then you will need to create keys in order access the API.

In [Projects]({{site.baseurl}}/docs/configuration/projects/), you create different projects, which allow you to share services and resources to specific groups of users. These projects are how your company can manage different teams and isolate resources between those teams.

In [Registries]({{site.baseurl}}/docs/configuration/registries/), you can add different private registry credentials so you can use your own images to create containers.

