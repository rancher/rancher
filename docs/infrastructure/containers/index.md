---
title: Containers
layout: default
---

## Containers
---

### Adding Containers

Typically, we recommend that people add containers using [services]({{site.baseurl}}/docs/services/projects/adding-services) as it provides a little more flexibility for the user, but sometimes we understand that you might want to spin up one container. 

There are a couple of ways to add a container to Rancher.

Option 1: In the **Infrastructure** -> **Container** page, click on the **Add Container**.

Option 2: On a specific host, you can click on the **+ Add Container** image within the list of containers on the host. 

These options will bring you to the **Add Container** page. Any options that `docker run` supports when creating containers is also supported in Rancher.

1. Provide a **Name** and if desired, **Description** for the host.
2. Provide the **Image** to use. You can use any image on [DockerHub](https://hub.docker.com/) as well as any [registries]({{site.baseurl}}/docs/configuration/registries) that have been added to Rancher. The syntax for image name would match any `docker run` commands. 

    Syntax of image names. By default, we pull from the docker registry. If no tag is specified, we will pull the latest tag. 

    `[registry-name]/[namespace]/[imagename]:[version]`

3. If desired, set up port mapping for your host to container relationship. Assuming that your host is using its public IP, when we are mapping ports, we are creating the ability to access the container through the host IP. In the **Port Map** section, you will define the public ports that will be used to communicate with the container. You will also be defining which port will be exposed on the container. When mapping ports for a container to a host, Rancher will check to see if there are any port conflicts. 

4. In the **Advanced Options** section, all options available in Docker are available for Rancher. By default, we have set the `-i -t`. 
  
    If you chose to add the container from the **Infrastructure** -> **Containers** page, Rancher will automatically pick a host for you. Otherwise, if you have picked a host to add a container to, the host will be populated within the **Advanced Options** -> **Security/Host** tab.
    
    **Labels/Scheduling**

    When creating containers, we provide the option to create labels for the container and the ability to schedule which host you want the container to be placed on. The scheduling rules provide flexibility on how you want Rancher to pick which host to use. In Rancher, we use labels to help define scheduling rules. You can create as many labels on a container as you'd like. With multiple scheduling rules, you have complete control on which host you want the container to be created on. You could request that the container to be launched on a host with a specific host label, container label or name, or a specific service. These scheduling rules can help create blacklists and whitelists for your container to host relationships. 

    Labels can be found in the **Advanced Options** -> **Labels** section of page. To add scheduling rules, open the **Advanced Options** -> **Scheduling** section. 

    ![Services on Rancher 4]({{site.baseurl}}/img/rancher_add_services_4.png)

    **Option 1: Run _all_ containers on a specific host**
    By selecting this option, the container will be started on the same host. If your host goes down, then the container will also go down. Even if there is a port conflict, the container will be started.

    **Option 2: Automatically pick a host matching scheduling rules**
    By selecting this option, you have the flexibility to choose your scheduling rules. Any host that follows all the rules is a host that could have the container started on. You can add rules by clicking on the **+** button. 

    For each rule, you select a **condition** of the rule. There are 4 different conditions, which define how strict the rule must be followed. The **field** determines which field you want the rule to be applied to. The **key** and **value** are the values which you want the field to be checked against. Rancher will spread the distribution of containers on the applicable hosts based on the load of each host. Depending on the condition chosen will determine what the applicable hosts are.

    _Conditions_
    * **must** or **must not**: Rancher will only pick a host that matches or does not match the field and value. If port mapping is defined on the container and there is no available host with those ports open, the container will fail to launch.
    * **should** or **should not**: Rancher will attempt to use a host that matches the field and value. In the case of when port mapping is defined and there is no host that satisfies the _should_ or _should not_ rules, Rancher will start ignoring 1 of these rules at a time to find a host.
    
    _Fields_
    * **host label**: When selecting the hosts to use for the service, Rancher will check the labels on the host to see if they match the key/value pair provided. Since every host can have one or more labels, Rancher will compare the key/value pair against all labels on a host. When adding a host to Rancher, you can add labels to the host. You can also edit the labels on the hosts by using the **Edit** option in the host's dropdown menu. The list of labels on active hosts are available from the dropdown in the key field.
    * **container with label**: When selecting this field, Rancher will look for hosts that already have containers with labels that match the key/value pair. Since every container can have one or more labels, Rancher will compare the key/value pair against all labels on every container in a host. The container labels are in the **Advanced Options** -> **Labels** for a container. You will not be able to edit the container labels after the container is started. In order to create a new container with the same settings, you can **Clone** the container and add the labels before starting it. The list of user labels on running containers are available from the dropdown in the key field.
    * **service with the name**: Rancher will check to see if a host has a container from the specified service running on it. If at a later time, this service has a name change or is inactive/removed, the rule will no longer be valid. If you pick this field, the value will need to be in the format of `project_name/service_name`. The list of running services are available from the dropdown in the value field.
    * **container with the name**: Rancher will check to see if a host has a container with a specific name running on it. If at a later time, the container has a name change or is inactive/removed, the rule will no longer be valid. The list of running containers are available from the dropdown in the value field.

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

When you select **Execute Shell**, it brings you into the container shell prompt. 

### Viewing Logs

It's always useful to see what is going on with the logs of a container. Clicking **View Logs** provides the equivalent of `docker logs <CONTAINER_ID>` on the host.


