---
title: Rancher Compose
layout: default

---

## Rancher Compose
---

The `rancher-compose` tool works just like the popular `docker-compose` and supports any `docker-compose.yml` file. When we launch a service in rancher-compose, it will show up in the specified Rancher server instance. We also have a `rancher-compose.yml` which extends and overwrites `docker-compose.yml.` The rancher-compose yaml file are attributes only supported in Rancher, for example, scale of a service.

The binary can be downloaded directly from the UI. The link can be found on the **Services** -> **Projects** page in the upper right corner. We have binaries for Windows, Mac, and Linux.

To enable `rancher-compose` to launch services in a Rancher instance, you'll need to set a couple of environment variables:`RANCHER_URL`, `RANCHER_ACCESS_KEY`, and `RANCHER_SECRET_KEY`. The access key and secret key will be an [API key]({{site.baseurl}}/docs/configuration/api-keys/). 

```bash
# Set the url that Rancher is on
$ export RANCHER_URL=http://server_ip:8080/
# Set the access key, i.e. username
$ export RANCHER_ACCESS_KEY=<username_of_key>
# Set the secret key, i.e. password
$ export RANCHER_SECRET_KEY=<password_of_key>
```

Now, you can create run any `docker-compose.yml` file using `rancher-compose`. The containers will automatically be launched in your Rancher instance in the [environment]({{site.baseurl}}/docs/configuration/environments/) that the API key is located in.

### Commands

`rancher-compose` supports any command that `docker-compose` supports.

Name | Description
----|-----
create	| Create all services but do not start
up		| Bring all services up
start	| Start services
logs	| 	Get service logs
restart	| Restart services
stop, down |	Stop services
scale	| Scale services
rm		| Delete services
help, h	| Shows a list of commands or help for one command


### Options

Whenever you use the `rancher-compose` command, there are different options that you can use. 

Name | Description
--- | ---
--debug	|		
--url 			|	Specify the Rancher API endpoint URL [$RANCHER_URL]
--access-key 	|		Specify Rancher API access key [$RANCHER_ACCESS_KEY]
--secret-key 	|		Specify Rancher API secret key [$RANCHER_SECRET_KEY]
--file, -f "docker-compose.yml"	| Specify an alternate compose file (default: docker-compose.yml)
--rancher-file, -r 		|	Specify an alternate Rancher compose file (default: rancher-compose.yml)
--project-name, -p 		|	Specify an alternate [project]({{site.baseurl}}/docs/services/projects/) name (default: directory name)
--help, -h			|	show help
--version, -v		|	print the version


### Compose Compatibility

`rancher-compose` strives to be completely compatible with Docker Compose.  Since `rancher-compose` is largely focused on running production workloads some behaviors between Docker Compose and Rancher Compose are different.

We support anything that can be created in a standard [docker-compose.yml](https://docs.docker.com/compose/yml/) file. There are a couple of differences in the behavior of rancher-compose that are documented below.

#### Deleting Services/Container

`rancher-compose` will not delete things by default.  This means that if you do two `up` commands in a row, the second `up` will do nothing.  This is because the first up will create everything and leave it running.  Even if you do not pass `-d` to `up`, `rancher-compose` will not delete your services.  To delete a service you must use `rm`.

#### Builds

Docker builds are supported in two ways.  First is to set `build:` to a git or HTTP URL that is compatible with the remote parameter in https://docs.docker.com/reference/api/docker_remote_api_v1.18/#build-image-from-a-dockerfile.  The second approach is to set `build:` to a local directory and the build context will be uploaded to S3 and then built on demand on each node.

For S3 based builds to work you must [setup AWS credentials](https://github.com/aws/aws-sdk-go/#configuring-credentials).

### Custom Rancher Services

Custom Rancher services are configured by using a special image name in the compose template.  The image name is how rancher-compose knows to set up a Rancher service versus a normal service.

Service | Image Name
--------|-----------
Load Balancer | rancher/load-balancer-service
External Service | rancher/external-service
Alias/DNS Service | rancher/dns-service


#### Load Balancer

Sample configuration `docker-compose.yml`


```yaml
lb:
  image: rancher/load-balancer-service
  ports:
  # Load Balancer public port 80 to private port 81 using HTTP
  - 80:81
  # Load Balancer public port 9999 to private port 8888 using tcp
  - 9999:8888/tcp
  labels:
    # Put load balancer containers on hosts with label lb=true
    - "io.rancher.scheduler.affinity:host_label=lb=true"
```

Sample `rancher-compose.yml`

```yaml
lb:
  # Two load balancer containers
  scale: 2
  load_balancer_config:
    name: lb config
    # Cookie policy configurations
    app_cookie_stickiness_policy: 
      cookie: cookiename
      max_length: 1024
      prefix: false
      request_learn: true
      timeout: 3600000
      mode: path_parameters
    # Health check configurations
    health_check:
      port: 80
      interval: 2000
      unhealthy_threshold: 3
      request_line: GET /index.html HTTP/1.0
      healthy_threshold: 2
      response_timeout: 2000
```

#### External Service

Sample configuration `docker-compose.yml`

```yaml
db:
  image: rancher/external-service

redis:
  image: redis
```

Sample `rancher-compose.yml`

```yaml
db:
  external_ips:
  - 1.1.1.1
  - 2.2.2.2

# Override any service to become an external service
redis:
  image: redis
  external_ips:
  - 1.1.1.1
  - 2.2.2.2
```

#### Service Alias/DNS service

Sample configuration `docker-compose.yml`

```yaml
web:
  image: rancher/dns-service
  links:
  - web1
  - web2

web1:
  image: nginx

web2:
  image: nginx
```
