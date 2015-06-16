---
title: Balancer Configs on Rancher
layout: default
---

## Balancer Configs on Rancher
---

A load balancer config is a configuration used to set up a [load balancer]({{site.baseurl}}/docs/infrastructure/load-balancers) or [load balancer service]({{site.baseurl}}/docs/services/projects/adding-balancers). The load balancer config includes listener(s), a health check policy and cookie policies (i.e. stickiness). When a load balancer is created, the balancer config is used to create the HAProxy config in the HA Proxy software inside the load balancer agent container. A load balancer config can be used with multiple load balancers, but cannot be re-used with balancer services.

> **Note:** If any change is made to the balancer config, it will be propagated on all load balancers or balancer services using that load balancer config.

In any balancer config, you have the ability to edit the listeners, health check or stickiness. Let's review in detail what each one of them are.

The list of load balancer configs can be found in the **Infrastructure** --> **Balancer Configs**. This list will show the names of all the balancer configs in Rancher as well as how many listeners are in each config. The stickiness policy will be displayed as well as the load balancers/load balancer services using the load balancer config. 

### Listeners 

A listener is a process that listens for connection requests. It is a one to one mapping of a port for the sources (i.e. hosts) to a port for the targets (i.e. containers/external public IPs) with protocols established for each port.  An algorithm is also selected for each listener to determine which target should be used. HAProxy is the software that is installed on the load balancers. You can read more about different algorithm rules that are used by [HAProxy]( http://cbonte.github.io/haproxy-dconv/configuration-1.5.html).

> **Note:** Currently, the only algorithm supported in a [balancer service]({{site.baseurl}}/docs/services/projects/adding-balancers/) is the round robin algorithm. We are looking to support the other algorithms in the future. In a standalone [load balancer]({{site.baseurl}}/docs/infrastructure/load-balancers/), we support different algorithms.

Any load balancer config will need a listener. The listener is the mapping that allows the incoming traffic to be distributed to your targets. Without the listener, the traffic will not be distributed.

### Health Check 

Health Check is the policy that can be defined to determine if the target ports and target IPs are reachable. 

If **HTTP Check** is blank, then a tcp request is being made against the healthcheck **port** to confirm that the backend server is up. This port is the private port defined in your balancer configuration. You can choose to fill in a http uri in the **HTTP Check**, which will change the health check into a http request to the specificed http uri. 

The **Check Interval** is how often Rancher will check that the targets are still available. The default interval is 2000 ms. The **Timeout** is how long Rancher will wait for a response from the target before giving up. The default timeout is 2000 ms. 

A **Healthy Threshold** is the number of consecutive successful health checks that are required for Rancher to consider the server as operational. 

An **Unhealthy Threshold** is the number of consectutive unsuccessful health checks that are required for Rancher to consider the server as dead. 

### Stickiness

Stickiness is the cookie policy that you want to use for when using cookies of the website. 

The three options that Rancher provides are:

* **None**: This option means that no cookie policy is in place.
* **Use existing cookie**: This option means that we will attempt to use the application's cookie for stickiness policies configuration. 
* **Create new cookie**: This option means that the cookie will be defined outside of your application. This cookie is what the load balancer would set on requests and responses. This cookie would determine the stickiness policy. 

You can only select one of these three choices and by default, we have selected **None**.

## Adding New Balancer Configs
---

In the **Infrastructure** -> **Balancer Configs**, you can add new balancer configs by clicking on the **Add Config** button. The new balancer config can be used when creating a load balancer, which allows you to set up your balancer configs before creating the load balancer. 

Provide the **Name** and **Description**, if desired, for the balancer config. 

Determine the **listeners** to add to the balancer configs, determine the **health check** policy and select the cookie policy.

Click on **Create** to add the balancer config to your list of available balancer configs.

## Changing Balancer Configs
----

If at any time you want to change the balancer configuration of the [load balancer]({{site.baseurl}}/docs/infrastructure/load-balancers/) or [balancer services]({{site.baseurl}}/docs/services/projects/adding-balancers), you go to the **Infrastructure** -> **Balancer Configs** tab. You will be able to see all load balancer configurations as well as the load balancers that are using the configurations.

For the balancer config that you want to edit, click on the dropdown menu and select **Edit**. You can edit anything in the balancer configuration, including adding/removing listener ports, health check and stickiness. 

Any changes made to a balancer configuration will automatically be changed in the load balancer. 

## Deleting Balancer Configs
---

Once a balancer config is created, you can remove it from the Rancher instance. But you'll only be able to remove a config as long as it's not actively being used by a load balancer or balancer service. In the **Infrastructure** -> **Balancer Configs**, you can view the list of balancer configs in Rancher in your [environment]({{site.baseurl}}/docs/configuration/environments/).

In the list of balancer configs, you can see which load balancers and balancer services are using the balancer configs. If the balancer config has **None** listed in the **Used by** column, then the dropdown menu of the balancer config will have the option to **Delete**.
