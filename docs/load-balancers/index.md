---
title: Load Balancing on Rancher
layout: default
---

## Load Balancing on Rancher
---
If you want to set up a web application, you're probably considering adding a load balancer. Let’s walk through the process of how to set one up in Rancher. If you want to understand the specific components of our load balancer, view our [FAQs]({{site.baseurl}}/docs/faqs/load-balancers/).

### Setting up the Web Service 
We'll need to have a couple of hosts launched before setting up our load balancer. If you haven't set up your hosts, please follow our [guide]({{site.baseurl}}/docs/getting-started/hosts/) to launch some new hosts. In our example, we've launched 4 hosts.

**DO YOU WANT TO ADD DETAILS OF HOW TO ADD HOST OR JUST DIRECT THEM TO GUIDE?**

Besides our hosts, we'll need to have our web application launched as well. Please follow our [guide]({{site.baseurl}}/docs/getting-started/services/) if you haven't set up your web service. In our example, we have 6 containers in our service. 

**DO YOU WANT TO ADD DETAILS OF HOW TO ADD SERVICE OR JUST DIRECT THEM TO GUIDE?**

### Adding the Load Balancer

With our hosts and web service set up, we are ready to launch the load balancer. Go to the the **Balancing** icon on the sidebar. Click on **Balancers**. There are two ways to add a Load Balancer. By either clicking on the **+** next to the Load Balancer text at the top of the screen or clicking on the image that contains **Add Load Balancer**. 

**IMAGE NEEDED TO SHOW HOW TO ADD LOAD BALANCERS**
![Load Balancing on Rancher 1]({{site.baseurl}}/img/Rancher_lb1.png)

In the **Add Load Balancer** page, provide a **name**, description (optional field), select the hosts that were launched earlier ,and select the targets (i.e. containers in the service that was made earlier). For hosts and targets, click on the **+** button to add additional ones. If you need to remove one of the hosts/targets, please click on the **x** next to the dropdown.

**IMAGE NEEDED TO SHOW First half of ADD LOAD BALANCERS PAGE**
![Load Balancing on Rancher 2]({{site.baseurl}}/img/Rancher_lb2.png)

Finally, you'll need to select a **Configuration** for the Load Balancer. If you have existing configurations, you'll have the opportunity to re-use it. In our case, since this is our first load balancer, we'll have to create a new configuration. Within each configuration, you define the **Listeners**, **Health Check** and **Stickiness**.

In the **Listeners** tab, you define the listening ports that are used from the source (i.e. host) to  the target (i.e. service or group of containers) as well as the protocol for each port. The source port is the port that will be accessed publicly through the host. The target port is the private port that targets will use to communicate with the hosts. 

Besides the ports and protocol, you'll also pick the algorithm. This algorithm is how the load balancer will choose which target to use. Please read this [article](http://cbonte.github.io/haproxy-dconv/configuration-1.5.html) to read more about algorithms that is used by HAProxy. HAProxy is the software that is installed on our load balancers.

You must define at least one listener. Otherwise, the load balancer won't be very useful! In our example, we'll configure our listener for these ports.
Source Port: 8090; Protocol: http
Target Port: 80; Protocol: http
Algorithm: round robin

**IMAGE NEEDED TO SHOW LISTENERS Tab**
![Load Balancing on Rancher 3]({{site.baseurl}}/img/Rancher_lb3.png)

In the **Health Check** tab, you can define the health check policy for your load balancer. The health check is used to check if the host is still available. There is a default policy already set on the load balancer configuration.

In our example, we'll leave the health check policy as the default policy.

**IMAGE NEEDED TO SHOW HEALTH CHECK Tab**
![Load Balancing on Rancher 4]({{site.baseurl}}/img/Rancher_lb4.png)

In the **Stickiness** tab, you can select a cookie policy.  

In our example, we'll select **None**.'

**IMAGE NEEDED TO SHOW STICKINESS Tab**
![Load Balancing on Rancher 5]({{site.baseurl}}/img/Rancher_lb5.png)

Click on **Create**. That’s it! Your load balancer will be launching some load balancer agents on the selected hosts. After these agents are finished installing, we'll be ready to test out our load balancer.

### Testing our load-balanced Web application







