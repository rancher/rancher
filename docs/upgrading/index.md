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
    $ docker stop <container_name_of_original_server>
    ```

2. Create a rancher-data container. Since we start the rancher server container with `--restart=always`, any reboot will restart the old container. Note: This step can be skipped if you have already upgraded in the past and are upgrading to a newer version.
    
    ```bash
    $ docker create --volumes-from <container_name_of_original_server> --name rancher-data rancher/server
    ```

3. Pull the most recent image of Rancher Server. Note: If you skip this step and try to run the latest image, it will not automatically pull an updated image.

    ```bash
    $ docker pull rancher/server:latest
    ```

4. Run this command to start the Rancher Server container using the data from the rancher-data container. Any changes made in the new version will be reflected in this data container.

    ```bash
    $ docker run -d --volumes-from rancher-data --restart=always -p 8080:8080 rancher/server:latest
    ```

    > **Note:** If you set any environment variables in your original Rancher server setup, you'll need to add those environment variables in the command.

5. Stop or delete the original rancher server container. Note: if you only stop the container, the container will be restarted if your machine is rebooted. We recommend deleting after you have created your rancher-data container.

### Rancher Agents 

Each Rancher agent version is pinned to a Rancher server version. If you upgrade Rancher server and Rancher agents require an upgrade, it will automatically upgrade the agents.