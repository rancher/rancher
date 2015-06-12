---
title: Quick Start Guide
layout: default

---

## Quick Start Guide
---

In this section, we will create the most basic Rancher install: a single host installation that runs everything on a single Linux machine.

### Prepare a Linux host

Get a Linux host with 64-bit Ubuntu 14.04 running, which must have a kernel of 3.10+. You can use your laptop, a virtual machine, or a physical server. Make sure the Linux host has at least **1GB** memory.

To install Docker on the server, follow these instructions, which are simplified from the [Docker](https://docs.docker.com/installation/ubuntulinux/) documentation. 

```bash
#Verify that wget is installed
$ which wget
#If wget isn't installed, install it after updating your manager
$ sudo apt-get update
$ sudo apt-get install wget
#Get the latest Docker package
$ sudo wget -qO- https://get.docker.com/ | sh
# Verify Docker works
$ sudo docker version
```

### Start Rancher Server

You can start Rancher server by simply typing this command:

```bash
$ sudo docker run --restart=always -p 8080:8080 rancher/server
```

> **Note:** The command is purposely not running with `-d` so you can see the log output and can make sure the server is listening on 8080. If you prefer to not watch the log output, you can add `-d`.

It takes a couple of minutes for Rancher server to start up. In the logs, Rancher UI is up and running, when you see this in your screen. 

```bash
.... Startup Succeeded, Listening on port 8080	
```

The UI is exposed on port 8080. Go to http://server_ip:8080 and you will see Rancher UI. If you are running the browser on the same host running Rancher server, make sure you use host’s real IP, like http://192.168.1.100:8080 and not http://localhost:8080 or http://127.0.0.1:8080.

### Add Hosts

For simplicity we will just add the same host running the Rancher server. In real production deployments, you will typically have dedicated hosts running Rancher servers. 

To add the Rancher server host, access the UI and click **Infrastructure**, which will immediately bring you to the **Hosts** page. Click on the **Add Host**. Rancher will prompt you to select an IP address that the server will be reachable from all the hosts you want to add in the future. This is useful, for example, in installations where Rancher server will be exposed to the Internet through a NAT firewall or a load balancer. If your host has a private or local IP address like `192.168.*.*`, Rancher will print a warning asking you to make sure hosts can indeed reach the IP.

For now you can ignore these warnings as we will only add the Rancher server host itself. Click **Save**. You’ll be presented with a few options to add hosts from various cloud providers. Since we are adding the existing Rancher server host, we click the **Custom** option. Rancher will display a command line like this:

```bash
$ sudo docker run -d --privileged -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:v0.7.9 http://server_ip:8080/v1/scripts/DB121CFBA836F9493653:1434085200000:2ZOwUMd6fIzz44efikGhBP1veo
```

Since we are adding the host on the Rancher server host, we will need to edit the command and insert `-e CATTLE_AGENT_IP=server_ip` into the command. This command will set the IP for the host to use. The updated command will look like this:

```bash
$ sudo docker run -e CATTLE_AGENT_IP=server_ip -d --privileged -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:v0.7.9 http://server_ip:8080/v1/scripts/DB121CFBA836F9493653:1434085200000:2ZOwUMd6fIzz44efikGhBP1veo
```

Just copy, paste, and run this command in a shell terminal of the Rancher server host.

Now, if you click **Close** on the Rancher UI, you will be directed back to the **Infrastructure** -> **Hosts** view, you should see the Rancher server host registered.

### Create a Container through UI

In the newly added host, click **+ Add Container**. Provide the container a name like “first_container”. Leave the rest of the selection as default and click **Create**. You will see Rancher creates two containers. One is the **_first_container_** you specified. The other is a **_Network Agent_**, which is a system container created by Rancher to handle tasks such as cross-host networking, health checking, etc.

Regardless what IP address your host has, you will see both the **_first_container_** and **_Network Agent_** have IP addresses in the `10.42.*.*` range. Rancher has created this overlay network so containers can communicate with each other even if they reside on different hosts.

If you hover over the **_first_container_**, you will be able to perform management actions like stopping the container, viewing the logs, or accessing the console.

### Create a Container through Native Docker CLI

Now, run the following command from Linux shell on the Rancher server host:

```bash
$ docker run -it --name=second_container ubuntu
```

You will see **_second_container_** pop up in Rancher UI! If you terminate the container by exiting the shell you will see the stopped state reflected in Rancher UI immediately.

This is how Rancher works: it reacts to events that happen out of the band and just does the right thing to reconcile its view of the world with reality.

If you take a look at the IP address of **_second_container_**, you will notice that it is not in `10.42.*.*` range. It instead has the usual IP address assigned by the Docker daemon. That is the expected behavior of creating a Docker container through the CLI.

What if we want to create a Docker container through CLI and still give it an IP address from Rancher’s overlay network? We can accomplish that by specifying a label on the Docker command line:

```bash
docker run --it --label io.rancher.container.network=true ubuntu
```

The label io.rancher.container.network enables us to pass a hint through the Docker command line so Rancher will set up the container to connect to the overlay network.

Given Rancher’s ability to import existing containers automatically, you might wonder why you do not see the Rancher server container itself in the Rancher UI. To avoid confusion, Rancher does not automatically import server or agent containers created by Rancher.

### Create a Multi-Container Application through Rancher Compose

<span>Provide an example to create a simple multi-container application using Rancher Compose</span>
