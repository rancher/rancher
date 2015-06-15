---
title: Adding External Service
layout: default
---

## Adding External Service
---

You may have services hosted outside of Rancher that you want integrated with Rancher. You can add these services into Rancher by adding an external service. 

Inside your project, you add an external service by clicking on the dropdown icon next to the **Add Service** button. Select **External Service**. Alternatively, if you are viewing the projects at the project level, the same **Add Service** dropdown is visible for each specific project.

You will need to provide a **Name** and if desired, **Description** of the service. 

Add the target(s) that you want. The target will be an external IP. Finally, click **Create**.

The list of IPs that the alias is serving is shown in the service. Just like our services, you will need to have the service alias started before it is working.

### Adding/Removing targets

At any time, you can edit the targets in an external service. Click on the **Edit** within the service's dropdown menu. You will have the ability to add more targets or remove existing targets.


