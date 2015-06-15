---
title: Using Rancher through Native Docker CLI
layout: default

---

## Using Rancher through Native Docker CLI
---

Rancher integrates with the native docker CLI so that it can be used alongside other DevOps and Docker tools. At a high level, this means that if you start, stop, or destroy containers outside of Rancher, Rancher will detect those changes and update accordingly.

### Docker Event Monitoring

Rancher updates in real time by monitoring Docker events on all hosts. So when a container is started, stopped, or destroyed outside of Rancher (for example by executing `docker stop sad_einstein` directly on a host), Rancher will detect that change and update its states accordingly. 

> **Note:** One current limitation is that we wait until containers are started (not created) to import them to Rancher. Running `docker create ubuntu` will not cause the container to appear in the Rancher UI, but running `docker start ubuntu` or `docker run ubuntu` will.

You can observe the same Docker event stream that Rancher is monitoring by executing `docker events` on the command line of a host.

### Joining natively started containers to the Rancher network

You can start containers outside of Rancher and still have them join the Rancher managed network. This means that these containers can participate in cross-host networking. To enable this feature, add the `io.rancher.container.network` label with a value of true to the container when you created. Here's an example:

```bash
$ docker run -l io.rancher.container.network=true -itd ubuntu bash
```

To read more about the Rancher managed network and cross-host networking, please read about Rancher [Concepts]({{site.baseurl}}/docs/concepts/).

### Importing Existing Containers

Rancher also supports importing existing container upon host registration. When you register a host through the [custom command]({{site.baseurl}}/docs/infrastructure/hosts/custom/), any containers currently on the host will be detected and imported into Rancher.

### Periodically Syncing State

In addition to monitoring docker events in real time, Rancher periodically syncs state with the hosts. Every five seconds, hosts report all containers to Rancher to ensure the expected state in Rancher matches the actual state on the host. This protects against things like network outages or server restarts that might cause Rancher to miss Docker events. When syncing state in this fashion, the state of the container on the host will always be the source of truth. So, for example, if Rancher thinks a container is running, but it is actually stopped on the host, Rancher will update the container's state to stopped. It will not attempt to restart the container.
