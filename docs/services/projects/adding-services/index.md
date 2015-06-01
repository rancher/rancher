---
title: Adding Services
layout: default
---

## Adding Services
---

With Rancher, you can add multiple services in a project to make an application. With this guide, we'll assume you've already created a project and are ready to build your application. If your project is not set up yet, please follow our [guide]({{site.baseurl}}/docs/services/projects/) on creating a project. 

> **Note:** Before starting any services, you'll need to have a least 1 host launched in Rancher. Please follow our [guide]({{site.baseurl}}/getting-started/hosts) to learn how to launch a host.

We'll walk through how to create a Wordpress application linked to a MySQL database. Inside your project, you add a service by clicking the **Add Service** button. Alternatively, if you are viewing the projects at the project level, you can add a service to a project with the **Add Service** button on the specific project. 

![Services on Rancher 1]({{site.baseurl}}/img/rancher_add_services_1.png)

![Services on Rancher 2]({{site.baseurl}}/img/rancher_add_services_2.png)

You will need to provide a **Name** and if desired, **Description** of the service. Use the slider to select the number of containers you want launched for the container. 

Provide the **Image** to use. You can use any image on [DockerHub](https://hub.docker.com/) as well as any [registries]({{site.baseurl}}/configuration/registries) that have been added to Rancher. The syntax for image name would match any `docker run` commands. 

Syntax of image names. By default, we pull from the docker registry. If no tag is specified, we will pull the latest tag. 

`[registry-name]/[namespace]/[imagename]:[version]`

We'll start by creating our MySQL database service with only 1 container.

![Services on Rancher 3]({{site.baseurl}}/img/rancher_add_services_3.png)

### Service Options

Just like adding individual [containers]({{site.baseurl}}/docs/infrastructure/containers/), any options that `docker run` supports, Rancher also supports! Port mapping and service links are shown on the main page, but other options are within the **Advanced Options** section. <span class="highlight">Do we want to go over every possible option in Rancher and how it maps to docker?</span>


![Services on Rancher 4]({{site.baseurl}}/img/rancher_add_services_4.png)

Assuming that your host is using its public IP, when we are mapping ports, we are creating the ability to access the container through the host IP. In the **Port Map** section, you will define the public ports that will be used to communicate with the container. You will also be defining which port will be exposed on the container. When mapping ports for a container to a host, Rancher will check to see if there are any port conflicts. 

When using port mapping, if the scale of your service is more than the number of hosts with the available port, your service will be stuck in an activating state. The service will continue to try and if host/port becomes available, the container will start and finish activating.

If other services have already been created, you can add links to the service. Linking services will link all containers in one service to all containers in another service. It acts just like the `--link` functionality in a `docker run` command. 

> **Note:** Linking services and/or containers will not automatically populate any environment variables that is supported when linking containers. You will need to manually add the environment variables when launching the container. 

For the MySQL service, we'll need to add the `MYSQL_ROOT_PASSWORD` as an environment variable and provide the key and value.

![Services on Rancher 5]({{site.baseurl}}/img/rancher_add_services_5.png)

Final step is to click **Create**. Creating the service will not automatically start the service. This allows you to create multiple services and when your application is ready, you can start all services at once!

Now that we've launched our database, we'll add the Wordpress service to our project. This time, we'll launch 3 containers in our service using the Wordpress image. We will not expose any ports in our Wordpress service as we will want to load balance this application. Since we've already created the database service, we'll pick the database service in the **Service Links**.

![Services on Rancher 6]({{site.baseurl}}/img/rancher_add_services_6.png)

As mentioned earlier, linking our services/containers will not automatically pass through any environment variables. In docker, when linking the database, the `WORDPRESS_DB_HOST` and `WORDPRESS_DB_PASSWORD` environment variables are typically populated. In our case, we'll need to manually add these environment variables to our service. When linking services to each other, we can just use the name of the service and Rancher will automatically provide all the containers in that service. 

![Services on Rancher 7]({{site.baseurl}}/img/rancher_add_services_7.png)

Click on **Create** and our Wordpress app is ready to be started! In our wordpress app, it shows us that the database service is linked. 


### Starting Services

There are several ways to start services. You can immediately start it after creating the service by clicking on the **Start** link within the service or even using the **Start** option in the service's dropdown menu. You can also wait until after you have created all your services and start them all at once using the **Start Services** in the dropdown menu of the Project. 

![Services on Rancher 8]({{site.baseurl}}/img/rancher_add_services_8.png)


### Load Balancing Services

At this time, it would make sense to load balance our Wordpress service. Let's move on to how to [add a load balancer]({{site.baseurl}}/docs/services/projects/adding-balancers/) into our project.




