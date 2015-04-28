---
title: Services
layout: default
---

## Getting Started with Services
---

### What is a Service? 

A service is a group of containers of the same image. With Rancher, you can create multiple services in one environment. These services make it simple to scale any application. 

### Creating an Environment

Before deploying any services, you'll need to have a least 1 host launched in Rancher. Please follow our [guide]({{site.baseurl}}/getting-started/hosts) to learn how to launch a host.

Click on the **Services** icon in the sidebar.

**IMAGE NEEDED FOR SERVICES ICON**
![Services on Rancher 1]({{site.baseurl}}/img/Rancher_services1.png)

Before setting up your services, you'll need to create an environment. An environment is just a namespace where the groups of services are deployed. With each namespace, you can have multiple sets of services. 

**IMAGE NEEDED FOR Environment landing page**
![Services on Rancher 2]({{site.baseurl}}/img/Rancher_services2.png)

Click on **+ Add Environment**. Provide a **Name** and **Description**. Click on **Create**.

**IMAGE NEEDED FOR Add Environment page**
![Services on Rancher 3]({{site.baseurl}}/img/Rancher_services3.png)

After creating your environment, you will be taken to the newly created environment. 


### Adding Services

You can start adding a service by clicking either the **+** button next to the name of the environment or clicking on the **+ Add Service** image. 

**IMAGE NEEDED FOR Environment with highlighted of where add service is**
![Services on Rancher 4]({{site.baseurl}}/img/Rancher_services4.png)

Provide a **Name** and if desired, **Description** of the service. Use the slider to select the number of containers you want launched for the container. 

**IMAGE NEEDED FOR Name/Description/Scale for Service**
![Services on Rancher 5]({{site.baseurl}}/img/Rancher_services5.png)

Select the **Image** to use. You can use any image on [DockerHub](https://hub.docker.com/) as well as any [private registries]({{site.baseurl}}/registries) that have been added to Rancher.

**IMAGE NEEDED FOR Image for Service**
![Services on Rancher 6]({{site.baseurl}}/img/Rancher_services6.png)

You can select various options for the service including mapping ports, linking to other services and additional options. Anything that `docker run` supports, Rancher supports as well!

When you're ready, click on **Create**. Creating the service will not automatically start the service. Since an environment may have multiple services to complete your application, we wait until you are ready to have your services started.

### Starting Services

After you have added your service, you can start services in several ways. If you have multiple services in your environment, you can start all services by clicking on the **Start Services** button at the top of the environment. Or in the **Environments** side bar, you can select **Start Services** from the drop down menu.

**IMAGE NEEDED FOR Start Services button and for image of sidebar option of starting services**
![Services on Rancher 7]({{site.baseurl}}/img/Rancher_services7.png)

If you wanted to start only one service at a time, you can either click on the **Start** inside the individual service or click on the **Start** in the dropdown menu.

**IMAGE NEEDED FOR individual service (highlighting the start button or drop down)**
![Services on Rancher 8]({{site.baseurl}}/img/Rancher_services8.png)

### Exporting Configuration
As you create services, we simultaneously create a `docker-compose.yml` and `rancher.yml` file of your environment. The `docker-compose` yaml file could be used outside of Rancher to start the same set of services using the `docker-compose` commands. Read [here](https://docs.docker.com/compose/) for more information on `docker-compose`. 

The `rancher.yml` file is used to contain the additional information used by Rancher to start services. These fields are not supported inside the docker-compose file.

You can view the files by clicking on the file icon in the environment.

**IMAGE NEEDED FOR configuration view with file icon highlighted**
![Services on Rancher 9]({{site.baseurl}}/img/Rancher_services9.png)

You can export this configuration by clicking on the **Export Config** button at the top of your environment. 

**IMAGE NEEDED FOR export config**
![Services on Rancher 10]({{site.baseurl}}/img/Rancher_services10.png)

Exporting the configuration is also supported from the sidebar in the drop down menu of the environment that you wish to export from.

### Relational View 










