---
title: Adding Service Alias
layout: default
---

## Adding Service Alias
---

By adding a service alias, it provides easier flexibility when upgrading services. In our example, we could have an application, which is version 1. We'll name the service `AppV1`. We can create a service alias named `AppName` and link it to the `AppV1` service. If you have an updated application, you can create a `AppV2` service and add it to `AppName`. When you have completed all your testing of `AppV2`, you can then stop and eventually delete the `AppV2` service. Since the service alias is in place, there is no disruption to your application and you have easily upgraded your service without having to re-configure anything linking to the `AppName` service!

Inside your project, you add a service alias by clicking on the dropdown icon next to the **Add Service** button. Select **Service Alias**. Alternatively, if you are viewing the projects at the project level, the same **Add Service** dropdown is visible for each specific project.

You will need to provide a **Name** and if desired, **Description** of the service. The **Name** will be the service alias for the service that you select. 

Select the target(s) that you want to add the alias to. The list of available targets is any service that is already created in the project. Finally, click **Create**.

The list of services that the alias is serving is shown in the service view. Just like our services, you will need to have the service alias started before it is working.

### Adding/Removing services

At any time, you can edit the services in a service alias. Click on the **Edit** within the service's dropdown menu. You will have the ability to add more services to the alias or remove existing services.
