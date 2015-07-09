---
title: Quick Start Guide
layout: default

---

## Quick Start Guide
---

In this guide, we will create a simple Rancher install, which is a single host installation that runs everything on a single Linux machine.

### Prepare a Linux host

Provision a Linux host with 64-bit Ubuntu 14.04, which must have a kernel of 3.10+. You can use your laptop, a virtual machine, or a physical server. Please make sure the Linux host has at least **1GB** memory.

To install Docker on the server, follow these instructions, which are simplified from the [Docker](https://docs.docker.com/installation/ubuntulinux/) documentation. 

```bash
#Get the latest Docker package
$ wget -qO- https://get.docker.com/ | sh
# Verify that you have the latest version
$ sudo docker version
```

### Start Rancher Server

All you need is one command to launch Rancher server. After launching the container, we'll tail the logs to see when the server is up and running.

```bash
$ sudo docker run -d --restart=always -p 8080:8080 rancher/server
# Tail the logs to show Rancher
$ sudo docker logs -f containerid
```

It will take a couple of minutes for Rancher server to start up. When the logs show `.... Startup Succeeded, Listening on port 8080`, Rancher UI is up and running.

Our UI is exposed on port `8080`, so in order to view the UI, go to http://server_ip:8080. If you are running your browser on the same host running Rancher server, you will need to use the host’s real IP, like http://192.168.1.100:8080 and not http://localhost:8080 or http://127.0.0.1:8080.

> **Note:** Rancher will not have access control configured and your UI and API will be available to anyone who has access to your IP. We recommend configuring [access control]]({{site.baseurl}}/docs/configuration/access-control/).

### Add Hosts

For simplicity, we will add the same host running the Rancher server as a host in Rancher. In real production deployments, we recommend having dedicated hosts running Rancher server(s). 

To add a host, access the UI and click **Infrastructure**, which will immediately bring you to the **Hosts** page. Click on the **Add Host**. If access control is not configured, Rancher will prompt you to select an IP address. This IP address must be reachable from all the hosts that you will be adding. This is useful in installations where Rancher server will be exposed to the Internet through a NAT firewall or a load balancer. If your host has a private or local IP address like `192.168.*.*`, Rancher will print a warning asking you to make sure hosts can indeed reach the IP.

For now you can ignore these warnings as we will only add the Rancher server host itself. Click **Save**. You’ll be presented with a few options to add hosts from various cloud providers. Since we are adding the host that is running Rancher server, we'll click the **Custom** option. In the UI, Rancher will provide a command to use to add hosts.

```bash
$ sudo docker run -d --privileged -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:v0.7.9 http://172.17.0.3:8080/v1/scripts/DB121CFBA836F9493653:1434085200000:2ZOwUMd6fIzz44efikGhBP1veo
```

Since we are adding a host that is running Rancher server, we need to edit the command and insert `-e CATTLE_AGENT_IP=<server_ip>` into the command, where `<server_ip>` is the IP address of the Rancher server host. 

In our example, `<server_ip>` is `172.17.0.3`, we will update the command to add in setting the environment variable.

```bash
$ sudo docker run -e CATTLE_AGENT_IP=172.17.0.3 -d --privileged -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:v0.7.9 http://172.17.0.3:8080/v1/scripts/DB121CFBA836F9493653:1434085200000:2ZOwUMd6fIzz44efikGhBP1veo
```

Run this command in a shell terminal of the host that is running Rancher server. 

When you click **Close** on the Rancher UI, you will be directed back to the **Infrastructure** -> **Hosts** view. In a little bit, the host will automatically appear.

### Create a Container through UI

In the newly added host, click **+ Add Container**. Provide the container a name like “first_container”. You can just use our default settings and click **Create**. Rancher will start launching two containers on the host. One container is the **_first_container_** that we requested. The other container is a **_Network Agent_**, which is a system container created by Rancher to handle tasks such as cross-host networking, health checking, etc.

Regardless what IP address your host has, both the **_first_container_** and **_Network Agent_** will have IP addresses in the `10.42.*.*` range. Rancher has created this overlay network so containers can communicate with each other even if they reside on different hosts.

If you hover over the **_first_container_**, you will be able to perform management actions like stopping the container, viewing the logs, or accessing the container console.

### Create a Container through Native Docker CLI

Rancher will display any containers on the host even if the container is created outside of the UI. Create a container in the host's shell terminal.

```bash
$ docker run -it --name=second_container ubuntu:14.04.2
```

In the UI, you will see **_second_container_** pop up on your host! If you terminate the container by exiting the shell, the Rancher UI will immediately show the stopped state of the container.

Rancher reacts to events that happen out of the band and just does the right thing to reconcile its view of the world with reality. You can read more about using Rancher with the [native docker CLI]({{site.baseurl}}/docs/native-docker/).

If you look at the IP address of the **_second_container_**, you will notice that it is not in `10.42.*.*` range. It instead has the usual IP address assigned by the Docker daemon. This is the expected behavior of creating a Docker container through the CLI. 

What if we want to create a Docker container through CLI and still give it an IP address from Rancher’s overlay network? All we need to do is add a label in the command. 

```bash
$ docker run -it --label io.rancher.container.network=true ubuntu:14.04.2
```

The label `io.rancher.container.network` enables us to pass a hint through the Docker command line so Rancher will set up the container to connect to the overlay network.

<!--Given Rancher’s ability to import existing containers automatically, you might wonder why you do not see the Rancher server container itself in the Rancher UI. To avoid confusion, Rancher does not automatically import server or agent containers created by Rancher.-->

### Create a Multi-Container Application

We have shown you how to create individual containers and connect them to a cross-host network. Most real-world applications, however, are made out of multiple services, with each service made up of multiple containers. A WordPress application, for example, could consist of the following services:

1. A load balancer service. The load balancer redirects Internet traffic to the WordPress application.
2. A WordPress service consisting of two WordPress containers.
3. A database service consisting of one MySQL container.

The load balancer service links to the WordPress service, and the WordPress service links to the MySQL service.

In this section, we will walk through how to create and deploy the WordPress application in Rancher.

From the Rancher UI, click the **Applications** tab, and click on the **Get Started** button to add your first service. 

You will immediately be able to add a service. First, we'll create a database service called _database_ and use the mysql image. In the **Advanced Options**, add the environment variable `MYSQL_ROOT_PASSWORD=pass1`. Click **Create**. You will be immediately brought to a stack page, which will contain all the services. After the service is created, click on **Start** in the service.

Next, click on the **Add Service** to add another service. We'll add a WordPress service and link to the mysql service. Let's use the name, _mywordpress_, and use the wordpress image. We'll move the slider to have the scale of the service be 2 containers. In the **Service Links**, add the _database_ service and provide the name _mysql_. Just like in Docker, Rancher will link the necessary environment variables in the WordPress image from the linked database when you select the name as _mysql_. Click **Create**. After the service is created, click on **Start** in the service and you will see 2 containers being launched for this service.

Finally, we'll create our load balancer. Click on the dropdown menu icon next to the **Add Service** button. Select **Add Balancer Service**. Provide a name like _wordpresslb_ and select the target. The target will be _mywordpress_ service. Finally, set the public port on the host that you'll use to access the wordpress application and set the target port as `80`. Click **Create**. After the service is created, click on **Start** in the service.

Our multi-service application is now complete! Find the IP of the host that the load balancer is on. Open a browser to the `host_IP:public_port` and you should see the wordpress application.

### Create a Multi-Container Application using Rancher Compose

In this section, we will show you how to create and deploy the same WordPress application we created in the previous section using a command-line tool called `rancher-compose`. 

The `rancher-compose` tool works just like the popular `docker-compose` tool. It takes in the same `docker-compose.yml` file and deploys the application on Rancher. You can specify additional attributes in a `rancher-compose.yml` file which extends and overwrites the `docker-compose.yml` file.

In the previous section, we created a Wordpress application with a load balancer. If you had created it in Rancher, you can download the files directly from our UI by selecting **Export Config** from the stack's dropdown menu. The `docker-compose.yml` and `rancher-compose.yml` files would look like this:

**docker-compose.yml**

```yaml
mywordpress:
  tty: true
  image: wordpress
  links:
    database: mysql
  stdin_open: true
wordpresslb:
  ports:
  - 8090:80
  tty: true
  image: rancher/load-balancer-service
  links:
    mywordpress: mywordpress
  stdin_open: true
database:
  environment:
    MYSQL_ROOT_PASSWORD: pass1
  tty: true
  image: mysql
  stdin_open: true
```

**rancher-compose.yml**

```yaml
mywordpress:
  scale: 2
wordpresslb:
  scale: 1
  load_balancer_config:
    lb_cookie_stickiness_policy: null
    description: null
    name: wordpresslb config
    app_cookie_stickiness_policy: null
    health_check:
      port: null
      interval: 2000
      unhealthy_threshold: 3
      request_line: ''
      healthy_threshold: 2
      response_timeout: 2000
database:
  scale: 1
```

Download the `rancher-compose` binary from Rancher UI, which is located on the upper right corner of the **Applications** -> **Stacks** page. We provide the ability to download the binaries for Windows, Mac, and Linux.

If order for services to be launched in Rancher using `rancher-compose`, you will need to set some variables in `rancher-compose`. You will need to create an [API Key]({{site.baseurl}}/docs/configuration/api-keys/) in the Rancher UI. Click on the account icon and go to **Settings** -> **API & Keys**. Click on **Add API Key**. Save the username (access key) and password (secret key). Set up the environment variables needed for rancher-compose: `RANCHER_URL`, `RANCHER_ACCESS_KEY`, and `RANCHER_SECRET_KEY`.

```bash
# Set the url that Rancher is on
$ export RANCHER_URL=http://server_ip:8080/
# Set the access key, i.e. username
$ export RANCHER_ACCESS_KEY=<username_of_key>
# Set the secret key, i.e. password
$ export RANCHER_SECRET_KEY=<password_of_key>
```

Now, navigate to the directory where you saved `docker-compose.yml` and `rancher-compose.yml` and run the command.

```bash
$ rancher-compose -p NewWordpress up
```

In Rancher, a new stack will be created called **NewWordPress** with all of the services launched.
