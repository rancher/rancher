---
title: Scheduling with Rancher-Compose
layout: default

---

## Scheduling with Rancher-Compose
---

When creating services using `rancher-compose`, you can direct the host(s) of where the containers should be launched based on scheduling rules. This scheduling ability is available in the [UI]({{site.baseurl}}/docs/services/projects/services/#scheduling-services) and also support in `rancher-compose`. Rancher determines how to schedule a service's containers based on the `labels` defined in the `docker-compose.yml` file.

### Labels used in Docker-Compose

All of the labels in this section would be used in the `docker-compose.yml` file. Rancher defines the scheduling rules with 3 main components: conditions, fields and values. Conditions determine how strictly Rancher follows the rules. Fields are which items that are going to be compared against. Values are what you've defined on the fields. We'll talk broadly about these components before going into some examples.

#### Scheduling Conditions

When we write our scheduling rules, we have conditions for each rule, which dictates how Rancher uses the rule. An affinity condition is when we are trying to find a field that matches our value. An anti-affinity condition is when we are trying to find a field that does not match our value. 

To differentiate between affinity and anti-affinity, we add `_ne` to the label name to indicate that the label is **not** matching the field and values.

There are also hard and soft conditions of a rule.

A hard condition is the equivalent of saying **must** or **must not**. Rancher will only use hosts that match or do not match the field and value. If Rancher cannot find a host that meets all of the rules with these conditions, your service could get stuck in an _Activating_ state. The service will be continually trying to find a host for the containers. To fix this state, you can either edit the scale value of the service or add/edit another host that would satisfy all of these rules. 

A soft condition is the equivalent of saying **should** or **should not**. Rancher will attempt to use hosts that match the field and value. In the case of when there is no  host that matches all the rules, Rancher will remove one by one the soft contraints (should/should not rules) until a host satisfies the remaining constraints. 

To differentiate between the _must_ and _should_ conditions, we add `_soft` to our label name to indicate that the label is **should** try to match the field and values.
     
#### Fields

Rancher has the ability to compare values against host labels, container labels, container name, or service name. The label prefix is what Rancher uses to define which field will be evaluated.

Field | Label Prefix
---|---
Host Label | `io.rancher.scheduler.affinity:host_label`
Container Label/Service Name | `io.rancher.scheduler.affinity:container_label`
Container Name | `io.rancher.scheduler.affinity:container`

Notice how there is not a specific prefix for service name. When Rancher creates a service, system labels are added to all containers of the service to indicate the project and service name. More details on how to compare against a service name is located in the [container label section]({{site.baseurl}}/docs/rancher-compose/scheduling/#container-labels).

To create the key of our label, we start with a field prefix (e.g. `io.rancher.scheduler.affinity:host_label`) and based on the condition that we are looking for, we append the type of condition we want. For example, if we want the containers to be launched on a host that must not equal (i.e. `_ne`) to a host label value, the label key would be `io.rancher.scheduler.affinity:host_label_ne`. 

#### Values

You use the values to define what you want the field to be checked against. If you have a couple of values that you want to compare against for the same condition and field, you'll need to use only one label for the name of the label. For the value of the label, you'll need to use a comma separated list. If there are multiple labels with the same key (e.g. `io.rancher.scheduler.affinity:host_label_ne`), Rancher will overwrite any previous value with the last value that is used with the label key.

```yaml
labels: 
  io.rancher.scheduler.affinity:host_label: key1=value1, key2=value2
```

#### Global Service

Making a service into a global service is the equivalent of selecting **Always run one instance of this container on every host** in the UI. This means that a container will be started on any host in the [environment]{{site.baseurl}}/docs/configuration/environments/). If a new host is added to the environment, and the host fulfills the global service's host requirements, the service will automatically be started. 

Currently, we only support global services with [host labels]({{site.baseurl}}/docs/rancher-compose/scheduling/#host-labels) that are using the hard condition. This means that only labels that are related to `host_labels` will be adhered to when scheduling and it **must** or **must not** equal the values. Any other label types will be ignored.

Example `docker-compose.yml`:

```yaml
wordpress:
  labels:
    # Make wordpress a global service
    io.rancher.scheduler.global: 'true'
    # Make wordpress only run containers on hosts with a key1=value1 label
    io.rancher.scheduler.affinity:host_label: key1=value1
    # Make wordpress only run on hosts that do not have a key2=value2 label
    io.rancher.scheduler.affinity:host_label_ne: key2=value2
  image: wordpress
  links:
    - db: mysql
  stdin_open: true
```

#### Finding Hosts with Host Labels

When adding hosts to Rancher, you can add [host labels]({{site.baseurl}}/docs/infrastructure/hosts/#host-labels). When scheduling a service, you can leverage these labels to create rules to pick the hosts you want your service to be deployed on.

Examples for each condition type:

```yaml
labels:
  # Host MUST have the label `key1=value1`
  io.rancher.scheduler.affinity:host_label: key1=value1
  # Host MUST NOT have the label `key2=value2`
  io.rancher.scheduler.affinity:host_label_ne: key2=value2
  # Host SHOULD have the label `key3=value3`
  io.rancher.scheduler.affinity:host_label_soft: key3=value3
  # Host SHOULD NOT have the label `key4=value4`
  io.rancher.scheduler.affinity:host_label_soft_ne: key4=value4
```

<a id="container-labels"></a>
#### Finding Hosts with Container Labels

When adding containers or services to Rancher, you can add container labels. These labels can be used for the field that you want a rule to compare against. Reminder: This cannot be used if you set [global service]({{site.baseurl}}/docs/rancher-compose/scheduling/#global-service) to true.

> **Note:** If there are multiple values for container labels, Rancher will look at all labels on all containers on the host to check the container labels. The multiple values do not need to be on the same container on a host. 

Examples for each condition type:

```yaml
labels:
  # Host MUST have a container with the label `key1=value1`
  io.rancher.scheduler.affinity:container_label: key1=value1
  # Host MUST NOT have a container with the label `key2=value2`
  io.rancher.scheduler.affinity:container_label_ne: key2=value2
  # Host SHOULD have a container with the label `key3=value3`
  io.rancher.scheduler.affinity:container_label_soft: key3=value3
  # Host SHOULD NOT have a container with the label `key4=value4
  io.rancher.scheduler.affinity:container_label_soft_ne: key4=value4
```

**Service Name**
When `rancher-compose` starts containers for a service, it also automatically creates several container labels. Since checking for a specific container label is looking for a `key=value`, we can use these system labels as the key of our rules. Here are the system labels created on the containers when Rancher starts a service: 

Label | Value
----|-----
io.rancher.project.name | `${project_name}`
io.rancher.project_service.name | `${project_name}/${service_name}`

Note: When using the `io.rancher.project_service.name`, the value must be in the format of `project name/service name.

The macros `${project_name}` and `${service_name}` can also be used in the `docker-compose.yml` file and will be evaluated when the service is started.

Example of Service Name:

```yaml
labels:
  # Host MUST have a container from service name `value1`
  io.rancher.scheduler.affinity:container_label: io.rancher.project_service.name=projectname/servicename
```

#### Finding Hosts with Container Names

When adding containers to Rancher, you give each container a name. You can use this name as a field that you want a rule to compare against. Reminder: This cannot be used if you set [global service]({{site.baseurl}}/docs/rancher-compose/scheduling/#global-service) to true.

```yaml
labels:
  # Host MUST have a container with the name `value1`
  io.rancher.scheduler.affinity:container: value1
  # Host MUST NOT have a container with the name `value2`
  io.rancher.scheduler.affinity:container_ne: value2
  # Host SHOULD have a container with the name `value3`
  io.rancher.scheduler.affinity:container_soft: value3
  # Host SHOULD NOT have a container with the name `value4
  io.rancher.scheduler.affinity:container_soft_ne: value4
```

### Examples

#### Example 1:

A typical scheduling policy may be to try to spread the containers of a service across the different available hosts.  One way to achieve this is to use an anti-affinity rule to itself:

```yaml
labels: 
  io.rancher.scheduler.affinity:container_label_ne: io.rancher.project_service.name=${project_name}/${service_name}
```

Since this is a hard anti-affinity rule, we may run into problems if the scale is larger than the number of hosts available.  In this case, we might want to use a soft anti-affinity rule so that the scheduler is still allowed to deploy a container to a host that already has that container running.  Basically, this is a soft rule so it can be ignored if no better alternative exists.

```yaml
labels: 
io.rancher.scheduler.affinity:container_label_soft_ne: io.rancher.project_service.name=${project_name}/${service_name}
```

#### Example 2:

Another example may be to deploy all the containers on the same host regardless of which host that may be.  In this case, a soft affinity to itself can be used.

```yaml
labels: 
  io.rancher.scheduler.affinity:container_label_soft: io.rancher.project_service.name=${project_name}/${service_name}
```

If a hard affinity rule to itself was chosen instead, the deployment of the first container would fail since there would be no host that currently has that service running.

### Table of Scheduling Labels

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
