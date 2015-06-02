---
title: Containers
layout: default
---

## Containers
---

<span class="highlight">A container is where applications and it's dependencies are launched through Docker. </span>

### Adding Containers

Typically, we recommend that people add containers using [services]({{site.baseurl}}/docs/services/projects/adding-services) as it provides a little more flexibility for the user, but sometimes we understand that you might want to spin up one container. 

There are a couple of ways to add a container to Rancher.

Option 1: In the **Infrastructure** -> **Container** page, click on the **Add Container**.

Option 2: On a specific host, you can click on the **+ Add Container** image within the list of containers on the host. 

These options will bring you to the **Add Container** page. Any options that `docker run` supports when creating containers is also supported in Rancher.

1. Provide a **Name** and if desired, **Description** for the host.
2. Provide the **Image** to use. You can use any image on [DockerHub](https://hub.docker.com/) as well as any [registries]({{site.baseurl}}/configuration/registries) that have been added to Rancher. The syntax for image name would match any `docker run` commands. 

    Syntax of image names. By default, we pull from the docker registry. If no tag is specified, we will pull the latest tag. 

    `[registry-name]/[namespace]/[imagename]:[version]`

3. If desired, set up port mapping for your host to container relationship. Assuming that your host is using its public IP, when we are mapping ports, we are creating the ability to access the container through the host IP. In the **Port Map** section, you will define the public ports that will be used to communicate with the container. You will also be defining which port will be exposed on the container. When mapping ports for a container to a host, Rancher will check to see if there are any port conflicts. 
 
    There is also an ability to select **Publish all ports to a random host port**. <span class="highlight">Need to know what the outcome of what this does</span>

4. In the **Advanced Options** section, all options available in Docker are available for Rancher. By default, we have set the `-i -t`. 
    
    Linking containers will not automatically populate any environment variables that is supported when linking containers. You will need to manually add the environment variables when launching the container. Rancher supports the ability to copy and paste environment variable (i.e. `name=value`) pairs into any of the environment variable name fields. 

    * All keys and values are trimmed on both sides. Empty lines are ignored.
    * If there is an existing value for a `name` in the paste, the old value is overwritten.
    * A line with just a key (no "=") is allowed. If the entire paste has no "=" then it is not a special paste and the string just replaces the name you pasted into.
  
    If you chose to add the container from the **Infrastructure** -> **Containers** page, Rancher will automatically pick a host for you. Otherwise, if you have picked a host to add a container to, the host will be populated within the **Advanced Options** -> **Security/Host** tab.

    <span class="highlight">Do we want to go over every possible option in Rancher and how it maps to docker?</span>

5. When you have completed filling out your container options, click **Create**. If this is the first container on the host to be launched by Rancher, it will automatically deploy a container named _Network Agent_ in the Rancher UI. This container is what Rancher uses to allow containers between different hosts be able to communicate with each other. The _Network Agent_ runs using the `rancher/agent-instance` image. Rancher will automatically pull the correct version tag for this container.

## Editing Containers
---

From the dropdown of a container, you can select different actions to perform on a container. When viewing containers on a host or service, the dropdown icon can be found by hovering over the container name. In the **Infrastructure** -> **Containers**, the dropdown icon is only visible for containers that were created specifically on the hosts. Any containers created through a service will not display its dropdown icon. 

You can always click on the container name, which will bring you to the container details page. On that page, the dropdown menu is located in the upper right hand corner next to the state of the container.

When you select **Edit** from the dropdown menu, you will be able to change the name and description of the container. 

### Changing the Container States

When a container is in a **Running** state, you can **Stop** the container. This will stop the container on the host, but will not remove it. After the container is in the _Stopped_ state, you can select **Start** to have the container start running again. Another option is to **Restart** the container, which will stop and start the container in one step. 

You can **Delete** a container and have it removed from the host. 

### Executing the Shell

When you select **Execute Shell**, it brings you into the container shell prompt. <span class="highlight">What do we want to write about the shell console?</span>"

### Viewing Logs

It's always useful to see what is going on with the logs of a container. Clicking **View Logs** provides the equivalent of `docker logs -f <CONTAINER_ID>` on the host. <span class="highlight">Is this correct?</span>


