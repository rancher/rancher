---
title: Basic SSL Rancher Server Configuration
layout: default
---

## Installing Rancher Server With SSL
---

In order to run Rancher Server from an https url, you will need to terminate SSL with a proxy that is capable of setting headers. This outlines the steps for NGINX, but you could use other tools.

## Requirements
In addition to Rancher Server requirements, you will also need:

* Valid SSL certificate
* DNS entries configured

## Launching Rancher Server

In this configuration, all traffic will pass through the proxy and be sent over a Docker link to the Rancher Server container. There are alternative approaches that could be followed, but this approach is simple and translates well. 

Start the Rancher Server container

```
sudo docker run -d --restart=always --name=rancher-server \
-e "CATTLE_API_ALLOW_CLIENT_OVERRIDE=true" \
-e "CATTLE_HOST_API_PROXY_SCHEME=wss" rancher/server
```

*Note: This is assuming you will run your proxy in a container. If you are going to run a proxy from the host, you will need to expose port 8080* 

If you are converting an existing environment configured with a data volume or external DB, stop and remove the existing Rancher Server container and launch the new container with --volumes-from=<data container> or DB settings. 

## Nginx Configuration

We realize that some of these configurations are not ideal, and are working to make things simpler and open up different SSL termination options.

Your NGINX configuration will need to look something like this, this is showing the minimum configuration, and should be customized to meet best practices:

```
upstream rancher {
    server rancher-server:8080;
}

server {
    listen 443 ssl;
    server_name <server>;
    ssl_certificate <cert_file>;
    ssl_certificate_key <key_file>;

    proxy_buffering off;
    proxy_buffer_size 512;

    location / {
        proxy_set_header X-API-request-url $scheme://<host>$request_uri;
        proxy_pass http://rancher;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}

server {
    listen 80;
    server_name <server>;
    return 301 https://$server_name$request_uri;
}
```

The important things here are that we disable Nginx proxy_buffering. Enabling this causes the host pings to backup on the server, and forces the agent to reconnect.

The other is setting the X-API-request-url header. This header is what makes the API schema show the correct information. We are working to support standard X-Forwarded-* headers. 

## Settings

The UI should now load. You now need to visit the URL in your browser:

`https://<your server>/v1/settings/api.host`

Click the `Edit` button. Set the value equal to a space " " and save.







