---
title: Upgrading Rancher
layout: default
---

## Upgrading Rancher
---

Currently, upgrades are **NOT** officially supported between releases before we hit a GA release. Therefore, certain features might break in later versions as we enhance them. The procedure we follow when we upgrade is outlined below. We typically only go from one version to the next if we do upgrade.

Use the original Rancher Server container to be your DB server forever. Any changes that are made in the upgraded version will always be saved in the original Rancher Server container.

> **Note:** Do not remove the original Rancher Server container at any time! 

1. Find the container name of Rancher Server.
```
docker ps
````
2. Stop the container.
```
docker stop <container name of old server>
```
3. Optional: Backup the data.
* Create another container with `--volumes-from=<name_of_old_server>`
* Copy the contents of /var/lib/cattle to someplace safe.
* if you are running >v0.10.0 also copy /var/lib/mysql
** on containers >=v0.10.0 3306 is exposed and you could publish and create MySQL slaves.

4. Pull the most recent image of Rancher Server. Note: If you skip this step and try to run the latest image, it will not automatically pull an updated image.
```
docker pull rancher/server:latest
```
5. Run this command to start the Rancher Server container using the data from the original Rancher Server container. 

```bash
docker run -d --volumes-from=<container_name_of_old_server> --restart=always -p 8080:8080 rancher/server:<version>
```

You can also configure an external MySQL database server by setting these environment variables on the container. This allows you to decouple the server from the DB. Please read the [guidelines]({{site.baseurl}}/docs/running-rancher/#external-db) on how to set up an external database.


### Rancher Agents 

Each Rancher agent version is pinned to a Rancher server version. If you upgrade Rancher server and Rancher agents require an upgrade, it should automatically upgrade the agents.