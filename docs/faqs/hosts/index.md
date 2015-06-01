---
title: FAQS on Rancher
layout: default
---

## Hosts FAQs
---
### How does the host determine IP address and how can I change it?

When the agent connects to Rancher server, it auto detects the IP of the agent. Sometimes, the IP that is selected is not the IP that you want to use. You can override this setting and set the host IP to what you want. 

In order to update the IP address for a host, you will need to alter the registration command for the host. You will need to set the CATTLE_AGENT_IP to the IP address that you want to use. 

If you already have hosts running, you just need to rerun the agent registration command. If you have any containers existing on the host, please follow the upgrade instructions in order to have the containers remained on your host.

> **Note:** You should not update the IP of a host to the docker0 interface on the host machine. 

Typically, the registration command from the UI follows this template:
```bash
sudo docker run -d --privileged -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:v0.5.2 http://MANAGEMENT_IP:8080/v1/scripts/SECURITY_TOKEN
```
The command will need to be edited to include setting the CATTLE_AGENT_IP by adding the **-e CATTLE_AGENT_IP=x.x.x.x**

```bash
sudo docker run -d --privileged -v /var/run/docker.sock:/var/run/docker.sock –e CATTLE_AGENT_IP=x.x.x.x rancher/agent:v0.5.2 http://MANAGEMENT_IP:8080/v1/scripts/SECURITY_TOKEN
```
> **Note:** When override the IP address, if there are existing containers in the rancher server, those hosts will no longer to be able to ping the host with the new IP. We are working to fix this issue, but please update the IP address with caution.


### What are all the different states that my host can display?

There are many states shown for a host as we are trying to provide as much detail to the user as possible so that you can understand what's going on with the host.

* _Creating_: When adding a new host through the UI, Rancher has many steps that occur to be in touch with the cloud provider. In the host, there are many messages about the actions that are being performed. 
* _Active_: The host is up and running and communicating with Rancher. This state will be green to allow an easy check to see if there are any issues with hosts. In this state, you can add new containers and perform any actions on the containers. 
* _Deactivating_: While the host is being deactivated, the host will display this state until the deactivation is complete. Upon completion, the host will move to an _Inactive_ state.
* _Inactive_: The host has been deactivated. No new containers can be deployed and you will be able to perform actions (start/stop/restart) on the existing containers. The host is still connected to the Rancher server.
* _Activating_: From an _Inactive_ state, if the host is being activated, it will be in this state until it's back to an _Active_state.
* _Bootstrapping_: When launching a host from a cloud provider, the _rancher agent_ will be launched on the newly created VM during the bootstrapping state.
* _Removed_: The host has been deleted by the user. This is the state that will show up on the UI until Rancher has completed the necessary actions. 
* _Purged_: The host is only in this state for a couple of seconds before disappearing from the UI. 
* _Reconnecting_: The host has lost its connection with Rancher. Rancher will attempt to restart the communication.

### What happens if my host is deleted outside of Rancher?

If your host is deleted outside of Rancher, then Rancher server will continue to show the host until it’s removed. Typically, these hosts will show up in a _Reconnecting_ state and never be able to reconnect. You will be able to **Delete** these hosts to remove them from the UI. 

### Why does my host have this weird name (e.g. 1ph7)?

If you didn’t enter a name during host creation, the UI will automatically assign a name. This name is displayed on the host. 
