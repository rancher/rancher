---
title: Environments within Rancher
layout: default
---

## Sharing Environments
---

### What is an Environment?

An environment is a way to share deployments and resources with different sets of users. Within each environment, you have the ability to invite others so it makes it easy to collaborate with others. By adding users to your environment, they will have the also have the ability to create deployments and manage resources. 

> **Note:** Infrastructure resources cannot be shared across multiple environments. [Registries]({{site.baseurl}}/docs/configuration/registries/) and [API-Keys]({{site.baseurl}}/docs/configuration/api-keys/) are also environment specific.  

The first time that you log in to Rancher, you are working in the **Default** environment. This default environment can be renamed, shared with others, or you can create additional environments to share with users. The environment that you're working in is always displayed in the upper right corner of the screen.

### Adding Environments

To add new environments, you'll need to navigate to the **Manage Environments** page. There are a couple of ways to get to the page.

* In the environment drop down, the **Manage Environments** link is at the bottom of the list of environments. 
* In the account drop down, the **Environments** link is under the **Settings** section.

<img src="{{site.baseurl}}/img/rancher_environments_2.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<img src="{{site.baseurl}}/img/rancher_environments_3.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<p style="clear: both;">


After navigating to the **Environments** page, you will see a list of environments.

![Environments on Rancher 4]({{site.baseurl}}/img/Rancher_environments_4.png)

Click on **Add Environment**. Each environment will have its own name, description, and members. The members can be any individual GitHub user, a GitHub organization, or a GitHub team. 

> **Note:** If you have not configured [Access Control]({{site.baseurl}}/docs/configuration/access-control/), all environments will be available to anyone accessing Rancher. There will be no restriction of membership for any environments.

There are two ways to add members to an environment. Provide the GitHub user or organization name. Click on the **+** to add the name to the list of members. If the name is not on the list, then they will not be added to the environment. Alternatively, there is a dropdown button on the right side of the **+** button. Rancher has automatically populated all the GitHub organizations and teams that your GitHub account is linked to. By selecting one of the organizations/teams, it will automatically add them to the list of members. 

For each member (i.e. individual, team, or organization), you can define the role to be either an owner or a member. By default, they are added as a member. You can change their role in the dropdown next to their name. If you are an owner, you can always change the list of members and their roles at any time.

Click on **Create** and the environment will immediately be available to all members and owners. At this point, anyone, that the environment is shared with, can start adding [services]({{site.baseurl}}/docs/services/)!

### Editing Environments

After creating environments, owners might want to deactivate or delete the environment. 

Deactivating an environment will remove the viewing ability from any members of the environment. All owners will still be able to view and activate the environment. You will not be able to change the membership of the environment until it's been re-activated. Nothing will change with your services or infrastructure. Therefore, if you want to make any changes to your services/infrastructure, you'll need to make these changes before your environment is deactivated.

In order to delete an environment, you will need to first deactivate it. When you delete an environment, any resources created by Rancher should be deleted. Any hosts launched through the UI will be deleted on the cloud provider, but any hosts added by the [custom host]({{site.baseurl}}/docs/infrastructure/hosts/custom/) will remain on the cloud provider. Since those were not launched by Rancher, Rancher does not have the ability to delete those hosts.

All registries, balancers and API keys created in the environment will be removed from Rancher.

### Editing Members

Owners can change the membership to an environment at any time. If an environment is a deactivated state, owners can still edit the membership of it. In the **Manage Environments** page, they will be able to **Edit** the environment. In the edit page, they will be able to add additional members by finding their names and clicking on the **+** button or selecting a name from the dropdown menu. 

If there are any members that you want to delete, click on the **X** next to their name in the list of members. Remember that if you delete an individual user, they could still have access to the environment if they are part of a team or organization that is a member of the environment.  

Owners can also change the roles of anyone on the member list. Just select the role that you want for the particular user.


