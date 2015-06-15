---
title: Host Registration  
layout: default
---

## Host Registration
---
Before launching any hosts, you will be asked to complete the host registration. This registration sets up how your Rancher environment is going to connect with your hosts. If you have already set up [access control]({{site.baseurl}}/docs/configuration/access-control), you will not be prompted to set host registration as Rancher assumes that your URL will be accessible.

![Host Registration on Rancher 1]({{site.baseurl}}/img/rancher_hosts_registration_1.png)

The setup determines what DNS name or IP address, and port that your hosts will be connected to the Rancher API. By default, we have selected the management server IP and port `8080`.  If you choose to change the address, please make sure to specify the port that should be used to connect to the Rancher API. This registration set up determines what the command will be for [adding custom hosts]({{site.baseurl}}/docs/infrastructure/hosts/custom/).

If [access control]({{site.baseurl}}/docs/configuration/access-control/) is turned on for Rancher, only the **admin** will be able to change the host registration. The **admin** is the first user to authenticate with Rancher when access control was configured. If access-control is still not configured, any users to the site can change host registration. In the account dropdown menu at the upper right corner, **Host Registration** can be found under the **Administration** section. 

