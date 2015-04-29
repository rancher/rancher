---
title: FAQS on Rancher
layout: default
---

## Hosts FAQs
---
**How do I add a new Host?**

Please follow the getting started [guide]({{site.baseurl}}/docs/getting-started/hosts/) on hosts to add new hosts. 

***What are all the different states that my host can display?***

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

**How do I remove a host from my Rancher server?**

In order to remove a host from the server, you will need to do a couple of step located in the host’s drop down menu. In order to view the drop down, hover over the host and a drop down icon will appear.

1. Select **Deactivate**. 
1. When the host has completed the deactivation, the host will display an _Inactive_ state. Select **Delete**. The server will start the removal process of the host from the Rancher server instance.  
1. Notes: All containers including the Rancher agent will continue to remain on the host.  The first state that it will display after it’s finished deleting it will be _Removed_. It will continue to finalize the removal process and move to a _Purged_ state before immediately disappearing from the UI. 
1. Optional: Once the host ‘s status displays _Removed_, you can purge the host to remove it from the UI faster.  We have this option for the user so that if any errors occur during the removal process, it can be displayed in between the _Removed_ and _Purged_ states. 

**What happens when I deactivate my host?**

Deactivating the host will put the host into an _Inactive_ state. In this state, no new containers can be deployed. Any active containers on the host will continue to be active and you will still have the ability to perform actions on these containers (start/stop/restart). The host will still be connected to the Rancher server.

**How do I get my _Inactive_ host up again?**

In the host’s drop down menu, select **Activate**. The host will become _Active_ and the ability to add additional containers will become available. 

**What happens if my host is deleted outside of Rancher?**

If your host is deleted outside of Rancher, then Rancher server will continue to show the host until it’s removed. Typically, these hosts will show up in a _Reconnecting_ state and never be able to reconnect. You will be able to **Delete** these hosts to remove them from the UI. 

**Why does my host have this weird name (e.g. 1ph7)?**

If you didn’t enter a name during host creation, the UI will automatically assign a name. This name is displayed on the host. 