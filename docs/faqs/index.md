---
title: FAQS on Rancher
layout: default
---

## FAQs
---

### What version of Rancher am I running?

As of our Beta release, you can click on the cow in the upper left hand corner, which will display which version of Rancher you are running. 

If clicking on the cow doesn't work, then you have a version prior to Beta. We recommend upgrading to Beta as it's a much more stable release due to many bug fixes. To find out what version you're running if prior to Beta, do a `docker inspect` on the container to view the environment variables. 

### How do I run Rancher and my hosts behind a proxy?

In order to set up a HTTP proxy, you'll need to edit the Docker daemon to point to the proxy. You will need to update this for your server and any hosts added to Rancher. You'll need to edit the `/etc/default/docker` file to point to your proxy and restart Docker.

```bash
$ sudo vi /etc/default/docker
```

Within the file, edit the `#export http_proxy="http://127.0.0.1:3128/"` to have it point to your proxy. Save your changes and then restart docker. Restarting Docker is different on every OS. 

You'll need to add in environment variables in order for the Rancher agent to use the proxy.

Potential Environment Variables to Set:
* http_proxy
* https_proxy
* NO_PROXY (must be capitalized)

```bash
$ sudo docker run -d -e http_proxy=<proxyURL> -e https_proxy=<proxyURL> -e NO_PROXY=<proxyURL> --restart=always -p 8080:8080 rancher/server
```
### How do linked containers/services work in Rancher?

In Docker, linked containers (using `--link` in `docker run`) shows up in the /etc/hosts of the container it's linked to. In Rancher, we don't edit the /etc/hosts. Instead we run a DNS server that makes links work across hosts. The DNS server would respond with the correct IP.

This means that any images that rely on populating environment variables from the `--link` container will not work. You will need to add these environment variables when adding the service/container.

<a id="subnet"></a>

### The subnet used by Rancher is already used in my network. How do I change the subnet?

In order for Rancher to work with a new subnet, you will need to start with a fresh install of Rancher. Before adding any hosts, you will need to update the subnet table with the new subnet IDs.

Within the Rancher server VM, you will need to follow these steps to update the subnet tables.

```bash
$ docker exec -it SERVER_CONTAINER_ID bash
SERVER_CONTAINER_ID$ mysql
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 12
Server version: 5.5.41-0ubuntu0.14.04.1 (Ubuntu)

Copyright (c) 2000, 2014, Oracle and/or its affiliates. All rights reserved.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql>
```

Get the cattle databases.

```bash
mysql> use cattle;
Reading table information for completion of table and column names
You can turn off this feature to get a quicker startup with -A

Database changed
```

Confirm that you have access to the subnet table.

```bash
mysql> show tables; 
+-----------------------------------------------+
| Tables_in_cattle                              |
+-----------------------------------------------+
| DATABASECHANGELOG                             |
| DATABASECHANGELOGLOCK                         |
| account                                       |
| agent                                         |
| agent_group                                   |
| certificate                                   |
| cluster_host_map                              |
| config_item                                   |
| config_item_status                            |
| credential                                    |
| credential_instance_map                       |
| data                                          |
| environment                                   |
| external_handler                              |
| external_handler_external_handler_process_map |
| external_handler_process                      |
| generic_object                                |
| global_load_balancer                          |
| host                                          |
| host_ip_address_map                           |
| host_vnet_map                                 |
| image                                         |
| image_storage_pool_map                        |
| instance                                      |
| instance_host_map                             |
| instance_link                                 |
| ip_address                                    |
| ip_address_nic_map                            |
| ip_association                                |
| ip_pool                                       |
| load_balancer                                 |
| load_balancer_config                          |
| load_balancer_config_listener_map             |
| load_balancer_host_map                        |
| load_balancer_listener                        |
| load_balancer_target                          |
| mount                                         |
| network                                       |
| network_service                               |
| network_service_provider                      |
| network_service_provider_instance_map         |
| nic                                           |
| offering                                      |
| physical_host                                 |
| port                                          |
| process_execution                             |
| process_instance                              |
| resource_pool                                 |
| service                                       |
| service_consume_map                           |
| service_expose_map                            |
| setting                                       |
| storage_pool                                  |
| storage_pool_host_map                         |
| subnet                                        |
| subnet_vnet_map                               |
| task                                          |
| task_instance                                 |
| vnet                                          |
| volume                                        |
| volume_storage_pool_map                       |
| zone                                          |
+-----------------------------------------------+
62 rows in set (0.00 sec)
```

Update the subnet table. 

```bash
mysql> update subnet set network_address='10.41.0.0', start_address='10.41.0.2', end_address='10.41.255.250', gateway='10.41.0.1' where id=1;                             
Query OK, 1 row affected (0.01 sec)
Rows matched: 1  Changed: 1  Warnings: 0
```

After the subnet table has been updated, you can add hosts/containers to the Rancher server and it will use the new subnet ID for the containers.