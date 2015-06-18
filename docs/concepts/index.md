---
title: Concepts
layout: default

---

## Concepts
---

In this section we introduce the key concepts in Rancher. You should be familiar with these concepts before attempting to use Rancher.

### Users

Users govern who has the access rights to view and manage Rancher resources within their environment.  Rancher allows access for a single tenant but multi-user support can be enabled by integrating with GitHub's OAuth support for authorization.

Please read about [access control]({{site.baseurl}}/docs/configuration/access-control/) to enable GitHub authentication.

### Environments

All hosts and any Rancher resources (i.e. containers, load balancers, etc.) are created and belong to an environment.  Access control to who can view and manage these resources are then defined by the owner of the environment.  Rancher currently supports the capability for each user to manage and invite other users to their environment and allows for the ability to create multiple environments for different workloads.  For example, you may want to create a "dev" environment and a separate "production" environment with its own set of resources and limited user access for your application deployment.

[Access control]({{site.baseurl}}/docs/configuration/access-control/) will need to be set up before being able to [share environments]({{site.baseurl}}/docs/configuration/environments/) with users. 

### Hosts

Hosts are the most basic unit of resource within Rancher and is represented as any Linux server, virtual or physical, with the following minimum requirements:

* Any modern Linux distribution that supports Docker 1.6+.
* Must be able to communicate with the Rancher server via http or https through the pre-configured port (Default is 8080).
* Must be routable to any other hosts belonging to the same environment to leverage Rancher's cross-host networking for Docker containers.

Rancher also supports Docker Machine and allows you to add your host via any of its supported drivers.

Read the following to [add your first host]({{site.baseurl}}/docs/infrastructure/hosts) to Rancher.

### Networking

Rancher supports cross-host container communication by implementing a simple and secure overlay network using IPsec tunneling.  To leverage this capability, a container launched through Rancher must select "Managed" for its network mode or if launched through Docker, provide an extra label "--label io.rancher.container.network=true".  Most of Rancher's network features such as a load balancer or DNS service require the container to be in the managed network.

Under Rancher's network, a container will be assigned both a Docker bridge IP (172.17.0.0/16) and a Rancher managed IP (10.42.0.0/16) on the default docker0 bridge.  Containers within the same environment are then routable and reachable via the managed network.

**_Note:_** _The Rancher managed IP address will be not present in Docker meta-data and as such will not appear in the result of a Docker "inspect." This sometimes causes incompatibilities with certain tools that require a Docker bridge IP. We are already working with the Docker community to make sure a future version of Docker can handle overlay networks more cleanly._

### Service Discovery

Rancher adopts the standard Docker Compose terminology for services and defines a basic service as one or more containers  created from the same Docker image.  Once a service (consumer) is linked to another service (producer) within the same project, a DNS record mapped to each container instance is automatically created and discoverable by containers from the "consuming" service.  Other benefits of creating a service under Rancher include:

* Service HA - the ability to have Rancher automatically monitor container states and maintain a service's desired scale.
* Health Monitoring - the ability to set basic monitoring thresholds for container health.
* Add Balancer Services - the ability to add a simple load balancer service for your services using HAProxy
* Add External Services - the ability to add any-IP as a service to be discovered
* Add Service Alias - the ability to add a DNS record for your services to be discovered

Read more about [adding services]({{site.baseurl}}/docs/services/projects/adding-services/), [adding balancer services]({{site.baseurl}}/docs/services/projects/adding-balancers/), [adding external services]({{site.baseurl}}/docs/services/projects/adding-external-services/) or [adding service alias]({{site.baseurl}}/docs/services/projects/adding-service-alias/).

### Load Balancer

Rancher implements a managed load balancer service using HAProxy that can be manually scaled to multiple hosts.  A load balancer can be used to distribute network and application traffic to individual containers by directly adding them or "linked" to a basic service.  A basic service that is "linked" will have all its underlying containers automatically registered as load balancer targets by Rancher.

Read more about our [load balancers]({{site.baseurl}}/docs/infrastructure/load-balancers/) and the [load balancer configurations]({{site.baseurl}}/docs/infrastructure/balancer-configs/) that are used with load balancers.

### Distributed DNS Service

Rancher implements a distributed DNS service using our own light-weight DNS server coupled with a highly available control plane. Each healthy container is automatically added to the DNS service when linked to another service or added to a Service Alias. When queried by the service name, the DNS service returns a randomized list of IP addresses of the healthy containers implementing that service.

Because Rancher’s overlay networking provides each container with a distinct IP address, we do not need to deal with port mappings and do not need to handle situations like duplicated services listening on different ports. As a result, a simple DNS service is adequate for handling service discovery.

### Health Checks

Rancher implements a distributed health monitoring system by running an HAProxy instance on every host for the sole purpose of providing health checks to containers.  When health checks are enabled either on an individual container or a service,  each container is then monitored by up to three HAProxy instances running on different hosts. The container is considered healthy if at least one HAProxy instance reports a "passed" health check.

Rancher’s approach handles network partitions and is more efficient than client-based health checks. By using HAProxy to perform health checks, Rancher enables users to specify the same health check policy for DNS service and for load balancers.

Depending on the result of health checks, a container is either in a green or red state. A service is in green (or "up") state if all containers implementing that service are in a green state and alternatively, in a red (or "down") state if all containers are subsequently in a red state.  A service is in yellow (or "degraded") state if Rancher has detected that at least one of the containers is either in a red state or in the process of returning it to a green state.

### Service HA

Rancher constantly monitors the state of your containers within a service and actively manages to ensure the desired scale of the service.  This can be triggered when there are fewer (or even more) healthy containers than the desired scale of your service, a host becomes unavailable, a container fails, or being unable to meet a health check.

### Service Upgrade

Rancher supports the notion of service upgrades by allowing users to either load balance or apply a service alias for a given service.  By leveraging either Rancher features, it creates a static destination for existing workloads that require that service.  Once this is established, the underlying service can be cloned from Rancher as a new service, validated through isolated testing, and added to either the load balancer or service alias when ready.  The existing service can be removed when obsolete and all network or application traffic are then automatically distributed to the new service.

### Rancher Compose

Rancher implements and ships a command-line tool called rancher-compose that is modeled after docker-compose. It takes in the same docker-compose.yml templates and deploys the projects onto Rancher. The rancher-compose tool additionally takes in a rancher-compose.yml file which extends docker-compose.yml to allow specifications of attributes such as scale, load balancing rules, health check policies, and external links not yet currently supported by docker-compose.

Read more about how to use [rancher-compose]({{site.baseurl}}/docs/rancher-compose/).

### Projects

A Rancher project mirrors the same concept as a docker-compose project.  It also defines the scope of service discovery when linking services to one another and represents a group of services that make up a typical application or workload.

<!--
```bash
rancher-compose up -p app1
```

This command deploys the docker-compose.yml template in the current directory into app1. All services in the same project can link to each other through service discovery.
-->
### Container Scheduling

Rancher supports container scheduling policies that are modeled closely after Docker Swarm.  They include scheduling based on:

* port conflicts
* shared volumes
* host tagging
* shared network stack: --net=container:dependency
* strict and soft affinity/anti-affinity rules using both env var (Swarm) and labels (Rancher)

In addition, Rancher supports scheduling service triggers that allow users to specify rules such as on "host add" or "host label" to automatically scale services onto hosts with specific labels.

Read more about scheduling with [rancher-compose]({{site.baseurl}}/docs/rancher-compose/scheduling/), in the UI with [services]({{site.baseurl}}/docs/services/projects/adding-services/#scheduling-services), or in the UI with individual [containers]({{site.baseurl}}/docs/infrastructure/containers/#scheduling-containers).

<!--
### Sidekicks

Rancher implements a special scheduling directive for the sidekick pattern. If service A is a sidekick to service B, they must be scheduled and scaled in lock step. A service can have multiple sidekicks. The volumes-from directive only works between sidekicks. Sidekicks is somewhat similar to Kubernetes pods although it is limited to scheduling and does not imply namespace sharing. (Alena to review and add more details)
-->
