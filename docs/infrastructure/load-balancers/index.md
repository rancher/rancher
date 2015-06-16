---
title: Load Balancing on Rancher
layout: default
---

## Load Balancing on Rancher
---

Our load balancer distributes network traffic across a number of containers. This is a key requirement for developers and ops who want to deploy large-scale applications. Load balancing support is notoriously uneven across different clouds. By offering a uniform, simple, and powerful load balancer solution, Rancher makes it possible for Docker developers to port applications easily across multiple clouds, or to use different cloud providers as independent resource pools to host highly available applications.

The load balancer is created and managed by Rancher. For each host selected for a Load Balancer, a load balancer agent (i.e. container) is started and HAProxy software is installed in it. 

## Adding a Load Balancer
---

In the **Infrastructure** -> **Load Balancers** page, click on the **Add Balancer** button. 

1. Provide a **Name** and if desired, **Description** for the host.
2. Select the host(s) that you want the load balancer agents to run on. **_Please be sure there is no port conflicts on the hosts that you pick._** Otherwise, the load balancer will not be able to finish creating. It will continue to try adding the load balancer agent onto the host until the port has no conflicts.

    > **Note:** If there is a port conflict on the host, the load balancer will still start on the host. When there are port conflicts on the host, the last container to be launched on the host using the port will be what the host communicates with.

3. Select the target(s) that you want to load balance. The targets can be any containers or any external IP address that you want traffic distributed for. The algorithm picked in the **Listeners** section will determine which target should be used. 
4. Create a new balancer config or re-use an existing one. Please read more about balancer configs in our detailed [documentation]({{site.baseurl}}/docs/infrastructure/balancer-configs).
5. Click on **Create**.


## Editing a Load Balancer
---

From the **Infrastructure** -> **Load Balancers** page, you can view all the load balancers in the Rancher instance including any balancers added to a project. 

When you hover over the load balancer, you can see the dropdown icon in the upper right corner. Alternatively, you can click on the load balancer's name to go to the detailed page. In the upper right corner of the page, the dropdown icon is available.

From the dropdown menu, you can **Delete** a load balancer, which will remove it from Rancher. The other available option is to **Edit**, which allows you to change the name and description.

### Editing Hosts 

If you want to add or remove hosts to the load balancer, you will need to go to the detailed view of the load balancer by clicking on its name. By default, it will navigate to the **Hosts** tab of the load balancer.

To add hosts to the load balancer, click on **Add Hosts**. A pop-up will appear to allow you to pick additional hosts. Click on **Add** to add these hosts to Rancher.

To remove hosts from the load balancer, pick the host that you want removed and select **Remove Host** from the dropdown menu. The dropdown menu is located on the right side of the host name.

### Editing Targets

If you want to add or remove targets to the load balancer, you will need to go to the detailed view of the load balancer by clicking on its name. Navigate to the **Targets** tab of the load balancer.

To add targets to the load balancer, click on **Add Targets**. A pop-up will appear to allow you to pick additional targets. You can either add additional container or external IP addresses. Click on **Add** to add these targets to Rancher.

To remove targets from the load balancer, pick the target that you want removed and select **Remove Target** from the dropdown menu. The dropdown menu is located on the right side of the targets name.

### Editing Balancer Configurations

In **Infrastructure** -> **Balancer Configs**, find the balancer config that you want to edit. In the balancer config's dropdown menu, select **Edit**. 

In the Edit page, you'll be able to edit all fields related to a balancer config. 