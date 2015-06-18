---
title: Upgrading Rancher
layout: default
---

## Upgrading Rancher
---

Currently, upgrades are **NOT** officially supported between releases before we hit a GA release. Therefore, certain features might break in later versions as we enhance them. The procedure we follow when we upgrade is outlined below. We typically only go from one version to the next if we do upgrade.

Use the original Rancher Server container to be your DB server. Any changes that are made in the upgraded version will always be saved in the original Rancher Server container. Do not remove the original Rancher Server container! 


1. Stop the container.

    ```bash
    $ docker stop <container_name_of_old_server>
    ```

2. Pull the most recent image of Rancher Server. Note: If you skip this step and try to run the latest image, it will not automatically pull an updated image.

    ```bash
    $ docker pull rancher/server:latest
    ```

3. Run this command to start the Rancher Server container using the data from the original Rancher Server container. Any changes made in the new version will be reflected in the volumes of the original Rancher server container. Therfore, if you have already upgraded in the past, you will need to use the same container name that was used to upgrade in the past. Example: OriginalRancher is upgraded to Rancher1. If I want to upgrade Rancher1, I would use the OriginalRancher as my volumes from container.

    ```bash
    $ docker run -d --volumes-from=<container_name_of_old_server> --restart=always -p 8080:8080 rancher/server:latest
    ```

    > **Note:** If you set any environment variables in your original Rancher server setup, you'll need to add those environment variables in the command.

### Rancher Agents 

Each Rancher agent version is pinned to a Rancher server version. If you upgrade Rancher server and Rancher agents require an upgrade, it will automatically upgrade the agents.