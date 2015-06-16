---
title: Scheduling through Native Docker CLI
layout: default

---

## Scheduling with Rancher-Compose
---

Scheduling of a service's containers is defined via labels in the `docker-compose.yml` file.
The following labels can be used:

Label | Value | Description
----|-----|-----
io.rancher.scheduler.global | true | Specifies this service to be a global service
io.rancher.scheduler.affinity:host_label | key1=value1, key2=value2, etc... | Containers **must** be deployed to a host with the labels `key1=value1` and `key2=value2`
io.rancher.scheduler.affinity:host_label_soft | key1=value1, key2=value2 | Containers **should** be deployed to a host with the labels `key1=value1` and `key2=value2`
io.rancher.scheduler.affinity:host_label_ne | key1=value1, key2=value2 | Containers **must not** be deployed to a host with the label `key1=value1` or `key2=value2`
io.rancher.scheduler.affinity:host_label_soft_ne | key1=value1, key2=value2 | Containers **should not** be deployed to a host with the label `key1=value1` or `key2=value2`
io.rancher.scheduler.affinity:container_label | key1=value1, key2=value2 | Containers **must** be deployed to a host that has containers running with the labels `key1=value1` and `key2=value2`.  NOTE: These labels do not have to be on the same container.  The can be on different containers within the same host.
io.rancher.scheduler.affinity:container_label_soft | key1=value1, key2=value2 | Containers **should** be deployed to a host that has containers running with the labels `key1=value1` and `key2=value2`
io.rancher.scheduler.affinity:container_label_ne | key1=value1, key2=value2 | Containers **must not** be deployed to a host that has containers running with the label `key1=value1` or `key2=value2`
io.rancher.scheduler.affinity:container_label_soft_ne | key1=value1, key2=value2 | Containers **should not** be deployed to a host that has containers running with the label `key1=value1` or `key2=value2`
io.rancher.scheduler.affinity:container | container_name1,container_name2 | Containers **must** be deployed to a host that has containers with the names `container_name1` and `container_name2` running
io.rancher.scheduler.affinity:container_soft | container_name1,container_name2 | Containers **should** be deployed to a host that has containers with the names `container_name1` and `container_name2` running
io.rancher.scheduler.affinity:container_ne | container_name1,container_name2 | Containers **must not** be deployed to a host that has containers with the names `container_name1` or `container_name2` running
io.rancher.scheduler.affinity:container_soft_ne | container_name1,container_name2 | Containers **should not** be deployed to a host that has containers with the names `container_name1` or `container_name2` running

### Rancher defined labels

When `rancher-compose` starts the containers for services, it also automatically defines several container labels.  These can be used in conjunction with the `io.rancher.scheduler.affinity:container_label` scheduling rules above.  Some examples are provided below.

Label | Value
----|-----
io.rancher.project.name | `${project_name}`
io.rancher.project_service.name | `${project_name}/${service_name}`

The macros `${project_name}` and `${service_name}` can be used in the `docker-compose.yml` file and will be evaluated when the service is started.

### Examples

#### Example 1:

A typical scheduling policy may be to try to spread the containers of a service across the different available hosts.  One way to achieve this is to use an anti-affinity rule itself:

    io.rancher.scheduler.affinity:container_label_ne: io.rancher.project_service.name=${project_name}/${service_name}

Since this is a hard anti-affinity rule, we may run into problems if the scale is larger than the number of hosts available.  In this case, we might want to use a soft anti-affinity rule so that the scheduler is still allowed deploy a container to a host that already has that container running.  Basically, this is a soft rule so it can be ignored if no better alternative exists.


#### Example 2:

Another example may be to deploy all the containers on the same host regardless of which host that may be.  In this case, a soft affinity to itself can be used.

    io.rancher.scheduler.affinity:container_label_soft: io.rancher.project_service.name=${project_name}/${service_name}

If a hard affinity rule to itself was chosen instead, the deployment of the first container would fail since there would be no host that currently has that service running.
