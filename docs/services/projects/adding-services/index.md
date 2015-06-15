---
title: Adding Services
layout: default
---

## Adding Services
---

With Rancher, you can add multiple services in a project to make an application. With this guide, we'll assume you've already created a [project]({{site.baseurl}}/docs/services/projects/), set up your [hosts]({{site.baseurl}}/docs/infrastructure/hosts/), and are ready to build your application. 

We'll walk through how to create a Wordpress application linked to a MySQL database. Inside your project, you add a service by clicking the **Add Service** button. Alternatively, if you are viewing the projects at the project level, the same **Add Service** button is visible for each specific project. 

You will need to provide a **Name** and if desired, **Description** of the service. In the **Scale** section, you can use the slider for the specific number of containers you want launched for a service. Alternatively, you can select **Always run one instance of this container on every host**. With this option, your service will scale for any additional hosts that are added to your [environment]({{site.baseurl}}/docs/configuration/environments/). Additionally, if you have scheduling rules in the **Advanced Options** -> **Scheduling**, Rancher will only start containers on the hosts that meet those conditions. If you add a host to your environment that doesn't meet the scheduling rules, a container will not be started on the host.

Provide the **Image** to use. You can use any image on [DockerHub](https://hub.docker.com/) as well as any [registries]({{site.baseurl}}/docs/configuration/registries) that have been added to Rancher. The syntax for image name would match any `docker run` commands. 

Syntax of image names. By default, we pull from the docker registry. If no tag is specified, we will pull the latest tag. 

`[registry-name]/[namespace]/[imagename]:[version]`

We'll start by creating our MySQL database service with only 1 container.

### Service Options

Just like adding individual [containers]({{site.baseurl}}/docs/infrastructure/containers/), any options that `docker run` supports, Rancher also supports! Port mapping and service links are shown on the main page, but other options are within the **Advanced Options** section. 

Assuming that your host is using its public IP, when we are mapping ports, we are creating the ability to access the container through the host IP. In the **Port Map** section, you will define the public ports that will be used to communicate with the container. You will also be defining which port will be exposed on the container. When mapping ports for a container to a host, Rancher will check to see if there are any port conflicts. 

When using port mapping, if the scale of your service is more than the number of hosts with the available port, your service will be stuck in an activating state. The service will continue to try and if host/port becomes available, the container will start and finish activating.

If other services have already been created, you can add links to the service. Linking services will link all containers in one service to all containers in another service. It acts just like the `--link` functionality in a `docker run` command. 

For the MySQL service, we'll need to add the `MYSQL_ROOT_PASSWORD` as an environment variable and provide the key and value. This field is located in the **Advanced Options**.

<a id="scheduling-services"></a>
### Labels/Scheduling 

By adding labels to a service, every container in the service will receive that label, which is a key value pair. In Rancher, we use container labels to help define scheduling rules. You can create as many labels on a container as you'd like. By default, Rancher already adds system related labels on every container. 

In a service, you might want to decide which hosts to have your containers started on. This can be accomplished by creating a set of scheduling rules to the service. To add scheduling rules, open the **Advanced Options** -> **Scheduling** section. 

![Services on Rancher 4]({{site.baseurl}}/img/rancher_add_services_4.png)

**Option 1: Run _all_ containers on a specific host**
By selecting this option, all containers in your service will be started on the same host. If your host goes down, then the service will also go down as there won't be any containers on any other hosts. 

**Option 2: Automatically pick hosts for each container matching scheduling rules**
By selecting this option, you have the flexibility to choose your scheduling rules. Any host that follows all the rules is a host that could have the containers started on. You can add rules by clicking on the **+** button. 

For each rule, you select a **condition** of the rule. There are 4 different conditions, which define how strict the rule must be followed. The **field** determines which field you want the rule to be applied to. The **key** and **value** are the values which you want the field to be checked against. Rancher will spread the distribution of containers on the applicable hosts based on the load of each host. Depending on the condition chosen will determine what the applicable hosts are.

_Conditions_

* **must** or **must not**: Rancher will only use hosts that match or do not match the field and value. If port mapping is defined on the service, the service could get stuck in an _Activating_ state when the number of containers in the service and available hosts. With port mapping, only one container can be launched per host to avoid port conflicts. Therefore, the service will continually try to find a host that could have the container started. In order to get the service out of the _Activating_ state, you can either edit the number on the scale by editing the service, or you can add another host that would satisfy the scheduling rules.
* **should** or **should not**: Rancher will attempt to use hosts that match the field and value. In the case of when port mapping is defined and there are not enough hosts that satisfy the _should_ or _should not_ rules, Rancher will start ignoring 1 of these rules at a time to find the correct number of hosts to finish launching the service. 

_Fields_

* **host label**: When selecting the hosts to use for the service, Rancher will check the labels on the host to see if they match the key/value pair provided. Since every host can have one or more labels, Rancher will compare the key/value pair against all labels on a host. When adding a host to Rancher, you can add labels to the host. You can also edit the labels on the hosts by using the **Edit** option in the host's dropdown menu.
* **container with label**: When selecting this field, Rancher will look for hosts that already have containers with labels that match the key/value pair. Since every container can have one or more labels, Rancher will compare the key/value pair against all labels on every container in a host. The container labels are in the **Advanced Options** -> **Labels** for a container. You will not be able to edit the container labels after the container is started. In order to create a new container with the same settings, you can **Clone** the container and add the labels before starting it.
* **service with the name**: Rancher will check to see if a host has a container from the specified service running on it. If at a later time, this service has a name change or is inactive/removed, the rule will no longer be valid. 
* **container with the name**: Rancher will check to see if a host has a container with a specific name running on it. If at a later time, the container has a name change or is inactive/removed, the rule will no longer be valid.

### Starting the Service

After filling out the information for your service, click **Create**. Creating the service will not automatically start the service. This allows you to create multiple services and when your application is ready, you can start all services at once!

Now that we've launched our database, we'll add the Wordpress service to our project. This time, we'll launch 3 containers in our service using the Wordpress image. We will not expose any ports in our Wordpress service as we will want to load balance this application. Since we've already created the database service, we'll pick the database service in the **Service Links** and select the name _mysql_. Just like Docker, Rancher will set up the environment variables in the WordPress image when linking two containers together, by naming the database as _mysql_.

Click on **Create** and our Wordpress app is ready to be started! In our wordpress app, it shows us that the database service is linked. 

### Starting Services

There are several ways to start services. You can immediately start it after creating the service by clicking on the **Start** link within the service or even using the **Start** option in the service's dropdown menu. You can also wait until after you have created all your services and start them all at once using the **Start Services** in the dropdown menu of the Project. 

### Load Balancing Services

At this point, it would make sense to load balance our Wordpress service. Let's move on to how to [add a balancer service]({{site.baseurl}}/docs/services/projects/adding-balancers/) into our project.
