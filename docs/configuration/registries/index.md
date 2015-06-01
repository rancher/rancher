---
title: Registries on Rancher
layout: default
---

## Registries 
---

With Rancher, you can add credentials to access private registries from DockerHub, Quay.io, or anywhere that you have a private registry. By having the ability to access your private registries, it enables Rancher to leverage your personal images. With Rancher, you can only add in one credential per registry address. This makes it a simple request to launch images from private addresses. 

At any time, you can view all the registries and the respective credentials. Click on the account icon in the upper right hand corner. A dropdown menu will appear with the different Rancher configuration settings. Within the **Settings** section, click on **Registries**. All registries that have been added will be listed in this Registries page. 

### Adding Registries

On the **Registries** page, click on **Add Registry**. 

For all registries, you'll need to provide the **e-mail address**, **username**, and **password**. For a **Custom** registry, you'll need to also provide the **registry address**. Click on **Create**.

### Using Registries
As soon as the registry is created, you will be able to use these private registries when launching services and containers. The syntax for the image name is the same as what you would use for the `docker run` command.

`[registry-name]/[namespace]/[imagename]:[version]`

By default, we are assuming that you are trying to pull images from `DockerHub`. 

### Editing Registries

All options for a registry are accessible through the dropdown menu on the right hand side of the listed registry.

For any **Active** registry, you can **Deactivate** the registry, which would prohibit access to the registry. No new containers can be launched with any images in that registry.

For any **Deactivated** registry, you have two options. You can **Activate** the registry, which will allow containers to access images from those registries. Any members of your environment will be able to activate your credential without needing to re-input the password. If you don't want anyone using your credential, you should **Delete** the registry, which will remove the credentials from the environment.

You can **Edit** any registry, which allows you to change the credentials to the registry address. You will not be able to change the registry address. The password is not saved in the "Edit" page, so you will need to re-input it in order to save any changes.

> **Note:** If a registry is deactivated or deleted, any [service]({{site.baseurl}}/docs/services/projects/adding-services/) or [container]({{site.baseurl}}/docs/infrastructure/containers/) using a private registry image will continue to run. If a service has been requested to scale up or is launching a new container to maintain its scale, the service would not be able to add new containers as the credentials are no longer available. 
