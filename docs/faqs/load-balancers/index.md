---
title: Load Balancer FAQS on Rancher
layout: default
---

## Load Balancer FAQs
---

**What is a Load Balancer?** 

Our Load Balancer distributes network traffic across a number of containers. This is a key requirement for developers and ops who want to deploy large-scale applications. Load balancing support is notoriously uneven across different clouds. By offering a uniform, simple, and powerful load balancer solution, Rancher makes it possible for Docker developers to port applications easily across multiple clouds, or to use different cloud providers as independent resource pools to host highly available applications.

The Load Balancer is created and managed by Rancher. For each host selected for a Load Balancer, a load balancer agent (i.e. container) is started and HAProxy software is installed in it. 

**DIAGRAM OF LOAD BALANCER/LISTENER/CONFIG**
![Load Balancing FAQs on Rancher 1]({{site.baseurl}}/img/Rancher_lbfaq1.png)

**What is a Load Balancer Configuration?**

A configuration that is used to set up the Load Balancer. The Load Balancer Config includes Load Balancer Listener(s), a Health Check Policy and cookie policies (i.e. Stickiness). When a Load Balancer is created, the Load Balancer Config is used to create the HAProxy config in the HA Proxy software inside the load balancer agent container. The Load Balancer Config can be used in multiple Load Balancers. 

Note: If any change is made to the Load Balancer Config, it will be propogated on all Load Balancers using that Load Balancer Config.

**What is a Load Balancer Listener?**

A Load Balancer Listener is a process that listens for connection requests. It is a one to one mapping of a port for the sources (i.e. hosts) to a port for the targets (i.e. containers/external public IPs) with protocols established for each port.  An algorithm is also selected for each listener to determine which target should be used. 

**Does my Load Balancer need a Load Balancer Listener?**

Yes, you can create a Load Balancer without a listener, but the LB will not be useful without a Load Balancer Listener. The Load Balancer Listener is the mapping that allows the incoming traffic to be distributed to your targets. Without this mapping, the Load Balancer will not be able to distribute the traffic to the targets. 

**What is a Load Balancer target?**

The target of a Load Balancer can be any containers or any external IP address that you want traffic distributed for.  The algorithm picked for the Load Balancer listeners will determine which target should be used.

**What is a Health Check?**

Health Check is the policy that can be defined to deteremine if the target ports and target IPs are reachable. 

**What is Stickiness?**

Stickiness is the cookie policy that you want to use for when using cookies of the website. 

**How do I change my Load Balancer?**

For each specific load balancer, the **Edit** capibility only displays changing the name and description. To make changes to how the Load Balancer will work, you'll need to edit the Load Balance Configuration, which can be found in the **Configurations** link in the **Balancing** page.

**IMAGE OF EDIT BUTTON FOR LOAD BALANCER**
![Load Balancing FAQs on Rancher 2]({{site.baseurl}}/img/Rancher_lbfaq2.png)


**How do I change my Load Balancer Confguration? What happens when I change a Load Balancer Config?**

By clicking on **Configurations** in the **Balancing** page, you will find the list of configurations used across all load balancers. In each configuration, you will be able to see the list of load balancers that are using this configuration. Click on the **Edit** in the drop down next to the configuration. 

**IMAGE OF EDIT BUTTON FOR LOAD BALANCER CONFIGURATION**
![Load Balancing FAQs on Rancher 3]({{site.baseurl}}/img/Rancher_lbfaq3.png)

After you've changed the configuration, the new config is propogated throughout the all Load Balancers using it and updates are made to the load balancer agent containers. This is all done automatically, so the user doesn’t have to worry about updating anywhere else in the system. 

