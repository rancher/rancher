---
title: Projects
layout: default
---

## Projects
---

<span class="highlight">WHAT IS A PROJECT?</span>

### Adding Projects

In the **Services** -> **Projects** page, click on **Add Project**. You will need to provide a **Name** and click **Create**. 

![Services Options on Rancher 1]({{site.baseurl}}/img/rancher_projects_1.png)


You will immediately be brought to the project and can begin [adding services]({{site.baseurl}}/docs/services/projects/adding-services/) or [adding balancers]({{site.baseurl}}/docs/services/projects/adding-balancers/).

> **Note:** Before deploying any services, you'll need to have a least 1 host launched in Rancher. Please follow our [documentation]({{site.baseurl}}/getting-started/hosts) to learn how to add hosts to Rancher.

### Viewing Project Services

From the projects page, you can easily monitor all your projects in your [environment]({{site.baseurl}}/docs/configuration/environment). From each project, you can expand the project to show the individual services by clicking on the carat next to the dropdown menu.

![Services Options on Rancher 2]({{site.baseurl}}/img/rancher_projects_2.png)


This will expand to show you any services within the project as well as all the containers that are part of the service. You can click on any individual container or service to go to the detailed page.

## Project Configuration
---

As services are created, we simultaneously create a `docker-compose.yml` and `rancher.yml` file of your project. The `docker-compose` yaml file could be used outside of Rancher to start the same set of services using the `docker-compose` commands. Read [here](https://docs.docker.com/compose/) for more information on `docker-compose`. 

The `rancher.yml` file is used to manage the additional information used by Rancher to start services. These fields are not supported inside the docker-compose file.

### Viewing Configurations

In the project dropdown, you can select **View Config** or click on the **file icon**.

<img src="{{site.baseurl}}/img/rancher_services_options_1.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<img src="{{site.baseurl}}/img/rancher_services_options_2.png" style="float: left; margin-right: 1%; margin-bottom: 0.5em;">
<p style="clear: both;">

### Exporting Configurations

There are a couple of options to export the files. 

Option 1: Download a zip file that contains both files by selecting **Export Config** in the project dropdown menu.

![Services Options on Rancher 3]({{site.baseurl}}/img/rancher_projects_3.png)

Option 2: Copy the file to your clipboard by clicking on the icon next to the file name that you want to copy. You can copy either the `docker-compose.yml` file or the `rancher-compose.yml` file. 

![Services Options on Rancher 4]({{site.baseurl}}/img/rancher_projects_4.png)

## Graph View 
---

We can view the project in another view, which shows how all services/balancers are related to each other. If they are linked together, there is a connection between the service names. 

Clicking on the **graph icon** will show this view.

<span class="highlight">**NEED IMAGE OF COMPLICATED APPLICATION!**</highlight>
![Services Options on Rancher 5]({{site.baseurl}}/img/rancher_projects_5.png)


### Editing Services/Balancers 
---

A service and balancer are created differently, but after creation, they are both treated as a service. All options for the services and balancers are the same. In the rest of this section, we will refer to just service, but the options apply to services and balancers.

### Scaling

You can quickly increase the number of containers in a service by clicking on **+ Scale Up** link. This link is located as an additional container in the service.

> **Note:** For balancer services, if you scale up to a quantity that is higher than the number of hosts with available public ports, the balancer will be stuck in **Updating-Active** state. You will need to start a new service if you need any of those type of changes. If it is stuck, the workaround is to **Stop** the balancer and change the scale back to the number of available hosts.

You can also increase or decrease the number of containers in a service by selecting **Edit** on the dropdown menu for the service. The dropdown menu is visible when hovering over the service. Move the slider for **Scale** to change the number containers in the service.

### Editing 
There are limited options for editing a service. To see what you can change, you select **Edit** on the dropdown menu of the service. The name, description and scale can be changed. If you forgot to link your service when you had set it up, you will have the ability to link services through this option. 

> **Note:** For container services, the advanced options and port mapping do not have the ability to change dynamically.

### Stopping 

You can stop individual services or all services in a project at once. If you want to stop an individual service, select **Stop** from the service dropdown menu. If you decide to stop all services in the project, you can select **Stop Services** from the project dropdown menu.

### Deleting

You can delete individual services/balancers or delete an entire project. When you select **Delete** for the individual service/balancer, it will stop the containers before removing them from the host. 

> **Note:** There might be a slight delay as we clean up the containers before they are deleted from the UI.

