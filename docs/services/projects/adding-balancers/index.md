---
title: Adding Balancers
layout: default
---

## Adding Balancers
---

After adding multiple services to your project, you might have decided that you want to load balancer your web applications. With Rancher, it's easy to add a balancer service to your project! 

We'll walk through how to load balance the Wordpress application created earlier in the [adding services guide]({{site.baseurl}}/docs/services/projects/adding-services/). Inside the Wordpress project, you add a balancer by clicking the **Add Service** dropdown icon and selecting **Balancer Service**. Alternatively, if you are viewing the projects at the project level, you can add a balancer service to a project with the **Add Service** dropdown on the specific project. 

In the **Add Load Balancer** page, you will need to provide a **Name** and if desired, **Description** of the load balancer. Use the slider to select the number of load balancers you want launched. 

> **Note:** The number of load balancers cannot exceed the number of hosts in the environment, otherwise there will be a port conflict and the balancer service will be stuck in an activating state. It will continue to try and find an available host and open port until you edit the scale of the balancer service or [add additional hosts]({{site.baseurl}}/infrastructure/hosts/). 

In our example, we only have 2 hosts in our environment, so therefore we can only create a maximum of 2 load balancers.

Next, we will select our target(s), which is our wordpress service. The available target(s) in the drop down list are any services within the project.

![Balancers on Rancher 3]({{site.baseurl}}/img/rancher_add_balancers_3.png)

In our load balancer, you will define a listener, health check and stickiness policy for the load balancer.

In the **Listeners** tab, you define the listening ports that are used from the source (i.e. host) to  the target (i.e. service or group of containers) as well as the protocol for each port. The source port is the port that will be accessed publicly through the host. The target port is the private port that targets will use to communicate with the hosts. Currently, the balancer services have only one algorithm that is used by HAProxy, which is round robin. HAProxy is the software that is installed on our balancers.

You must define at least one listener. Otherwise, the load balancer won't be very useful! In our example, we'll configure our listener with these settings:

* Source Port: 8090; Protocol: http
* Target Port: 80; Protocol: http
* Algorithm: round robin

In the **Health Check** tab, you can define the health check policy for your load balancer. The health check is used to help determine if a host is still available and the web service is still useable. There is a default policy already set on the balancer, but you can always edit it.

In the **Stickiness** tab, you can select a cookie policy. By default, there is no cookie policy selected. <span class="highlight">Need more details on stickiness</span>. 

![Balancers on Rancher 4]({{site.baseurl}}/img/rancher_add_balancers_4.png)

Click on **Create**. 

Just like with services, the balancer is not started until the user starts the service. You can individually start the load balancer by clicking **Start** or selecting **Start** in the dropdown menu. If the user selects to **Start Services** from the dropdown menu of the project, then it will also start a balancer.

Now, to see the balancer in action, click on a container name inside the balancer service. This will bring you to a detailed container page of the balancer. Copy the IP address of the host by clicking on the **paper icon**. Paste the IP into the web browser of your choice and add **:8090**. The Wordpress application will come up.

![Balancers on Rancher 5]({{site.baseurl}}/img/rancher_add_balancers_5.png)

