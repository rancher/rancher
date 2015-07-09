---
title: Projects within Rancher
layout: default
---

## Sharing Projects
---

### What is a Project?

A project is a way to share deployments and resources with different sets of users. Within each project, you have the ability to invite others so it makes it easy to collaborate with others. By adding users to your project, they will have the also have the ability to create deployments and manage resources. 

> **Note:** Infrastructure resources cannot be shared across multiple projects. [Registries]({{site.baseurl}}/docs/configuration/registries/) and [API-Keys]({{site.baseurl}}/docs/configuration/api-keys/) are also project specific.  

The first time that you log in to Rancher, you are working in a **Default** project. This project can be renamed, shared with others, or you can create additional projects to share with users. The project that you're working in is always displayed in the upper right corner of the screen.

### Adding Projects

To add new projects, you'll need to navigate to the **Manage Projects** page. There are a couple of ways to get to the page.

* In the project drop down, the **Manage Projects** link is at the bottom of the list of projects. 
* In the account drop down, the **Projects** link is under the **Settings** section.

<img src="{{site.baseurl}}/img/rancher_projects_1.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<img src="{{site.baseurl}}/img/rancher_projects_2.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<p style="clear: both;">

After navigating to the **Projects** page, you will see a list of projects.

Click on **Add Project**. Each project will have its own name, description, and members. The members can be any individual GitHub user, a GitHub organization, or a GitHub team. 

> **Note:** If you have not configured [Access Control]({{site.baseurl}}/docs/configuration/access-control/), all projects will be available to anyone accessing Rancher. There will be no restriction of membership for any projects.

There are two ways to add members to a project. Provide the GitHub user or organization name. Click on the **+** to add the name to the list of members. If the name is not on the list, then they will not be added to the project. Alternatively, there is a dropdown button on the right side of the **+** button. Rancher has automatically populated all the GitHub organizations and teams that your GitHub account is linked to. By selecting one of the organizations/teams, it will automatically add them to the list of members. 

For each member (i.e. individual, team, or organization), you can define the role to be either an owner or a member. By default, they are added to the list as a member. You can change their role in the dropdown next to their name. As an owner, you can always change the list of members and their roles at any time, but members do not have the ability to change the membership of the project.

Click on **Create** and the project will immediately be available to all members and owners. At this point, anyone, that the project is shared with, can start adding [services]({{site.baseurl}}/docs/services/)!. adding [hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/) and anything else that a member can do.

### Editing Projects

After creating projects, owners might want to deactivate or delete the project. 

Deactivating a project will remove the viewing ability from any members of the project. All owners will still be able to view and activate the project. You will not be able to change the membership of the project until it's been re-activated. Nothing will change with your services or infrastructure. Therefore, if you want to make any changes to your services/infrastructure, you'll need to make these changes before your project is deactivated.

In order to delete a project, you will need to first deactivate it. All registries, balancers and API keys created in the project will be removed from Rancher.

> **Note:** When deleting a project, the hosts will not be removed from the cloud providers, so please make sure to check your cloud provider after deleting a project. 

### Editing Members

Owners can change the membership to a project at any time. If a project is a deactivated state, owners can still edit the membership of it. In the **Manage Projects** page, they will be able to **Edit** the project. In the edit page, they will be able to add additional members by finding their names and clicking on the **+** button or selecting a name from the dropdown menu. 

If there are any members that you want to delete, click on the **X** next to their name in the list of members. Remember that if you delete an individual user, they could still have access to the project if they are part of a team or organization that is a member of the project.  

Owners can also change the roles of anyone on the member list. Just select the role that you want for the particular user.


