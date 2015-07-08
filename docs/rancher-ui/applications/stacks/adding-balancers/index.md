---
title: Adding Balancers
layout: default
---

## Adding Balancers
---

After adding multiple services to your stack, you might have decided that you want to load balancer your web applications. With Rancher, it's easy to add a balancer service to your stack! 

We'll walk through how to load balance the Wordpress application created earlier in the [adding services guide]({{site.baseurl}}/docs/rancher-ui/applications/stacks/adding-services/). Inside the Wordpress stack, you add a balancer by clicking the **Add Service** dropdown icon and selecting **Balancer Service**. Alternatively, if you are viewing the stacks at the stack level, you can add a balancer service to a stack with the **Add Service** dropdown on the specific stack. 

In the **Add Load Balancer** page, you will need to provide a **Name** and if desired, **Description** of the balancer service. Use the slider to select the scale, i.e. how many containers. 

> **Note:** The number of containers of this balancer service cannot exceed the number of hosts in the project, otherwise there will be a port conflict and this service will be stuck in an activating state. It will continue to try and find an available host and open port until you edit the scale of this balancer service or [add additional hosts]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/). 

In our example, we only have 2 hosts in our project, so therefore we can only create a maximum of 2 load balancers.

Next, we will select our target(s), which is our wordpress service. The available target(s) in the drop down list are any services within the stack.

In our balancer service, you will define balancer configuration, which consists of a listener, health check and stickiness policy. Please read more about [balancer configuration]({{site.baseurl}}/docs/rancher-ui/infrastructure/balancer-configs/) to understand the details of how you can configure your balancing service.

In the **Listeners** tab, you define the listening ports that are used from the source (i.e. host) to  the target (i.e. service or group of containers) as well as the protocol for each port. The source port is the port that will be accessed publicly through the host. The target port is the private port that targets will use to communicate with the hosts. Currently, the balancer services have only one algorithm that is used by HAProxy, which is round robin. HAProxy is the software that is installed on our balancers.

You must define at least one listener. Otherwise, the load balancer won't be very useful! In our example, we'll configure our listener with these settings:

* Source Port: `8090`; Protocol: http
* Target Port: `80`; Protocol: http
* Algorithm: round robin

Let's leave **Health Check** and **Stickiness** to the default values, but you can read more about these options in our [balancer configuration]({{site.baseurl}}/docs/rancher-ui/infrastructure/balancer-configs/).

Click on **Create**. 

Just like with services, the balancer is not started until the user starts the service. You can individually start the load balancer by clicking **Start** or selecting **Start** in the dropdown menu. If the user selects to **Start Services** from the dropdown menu of the stack, then it will also start a balancer.

Now, to see the balancer in action, click on a container name inside the balancer service. This will bring you to a detailed container page of the balancer. Copy the IP address of the host by clicking on the **paper icon**. Paste the IP into the web browser of your choice and add `:8090`. The Wordpress application will come up.

