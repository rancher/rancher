---
title: Environments within Rancher
layout: default
---

## Sharing Environments
---

### What is an Environment?

An environment is a way to share deployments and resources with different sets of users. Within each environment, you have the ability to invite others so it makes it easy to collaborate with others. By adding users to your environment, they will have the also have the ability to create deployments and manage resources.

Please note that resources cannot be shared across multiple environments. 

The first time that you log in to Rancher, you are already working in the **Default** environment. This default environment can be renamed, shared with others, or you can create additional environments to share with users. The environment that you're working in is always displayed in the upper right corner of the screen.

**IMAGE NEEDED FOR ENVIRONMENT MENU LOCATION**
![Environments on Rancher 1]({{site.baseurl}}/img/Rancher_environments1.png)

Let's walk through adding a new environment and sharing it!

### Creating and Sharing New Environments

To add new environments, you'll need to navigate to the **Manage Environments** page. There are a couple of ways to get to the page.

* In the environment drop down, the **Manage Environments** link is at the bottom of the list of environment. 
* In the account drop down, the **Manage Environment** link is under the **Settings** section.

**IMAGE NEEDED FOR MANAGE ENVIRONMENTS LOCATIONS (Side by Side Image)**
![Environments on Rancher 2]({{site.baseurl}}/img/Rancher_environments2.png)

After clicking on **Manage Environments**, the page will show the list of environments. On the right hand side, there is a blue button, **+ Add Environment**. Click on it to add a new environment!

**IMAGE NEEDED FOR MANAGE ENVIRONMENTS PAGE**
![Environments on Rancher 3]({{site.baseurl}}/img/Rancher_environments3.png)

Each environment will have its own name, description, and members. The members can be any individual GitHub user, a GitHub organization, or a GitHub team. In the dropdown, we've automatically populated all the GitHub organizations and teams that you belong to. This makes it easy to add any teams. 

For each member (i.e. individual, team, or organiation), you can define the role to be either an owner or a member. By default, they are added as a member. You can change their role in the drop down. If you are an owner, you can always change the list of members and their roles at any time.

In our example, I'm going to add the dev team my environment as members, but add an individual user on the dev team as an owner. This allows the individual user to have all rights as an owner. 

**IMAGE NEEDED FOR ADDING ENVIRONMENT PAGE WITH DETAILS ON TEAM NAME AND INDIVIDUAL USER**
![Environments on Rancher 4]({{site.baseurl}}/img/Rancher_environments4.png)

Click on **Create** and the environment will be available to all members and owners. At this point, anyone that the environment is shared with can start adding deployments!

### Setting up an Environment before Sharing 

In some cases, you will want to set up your deployments before sharing it with others. In that case, we recommend create your environment without sharing it. Add any deployments to the environment before sharing with others by adding members in the Manage Environments page. 


