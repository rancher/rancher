---
title: Basic SSL Rancher Server Configuration
layout: default
---

## Installing Rancher Server With SSL
---

In order to run Rancher Server from an https url, you will need to terminate SSL with a proxy that is capable of setting headers. We''ll provide an outline of how to set it up with NGINX, but other tools could be used. 

## Requirements

Besides the typical Rancher Server [requirements]({{site.baseurl}}/docs/installing-rancher/installing-server/#requirements), you will also need:

* Valid SSL certificate
* DNS entries configured

## Launching Rancher Server

In our configuration, all traffic will pass through the proxy and be sent over a Docker link to the Rancher Server container. There are alternative approaches that could be followed, but this approach is simple and translates well. 

Start the Rancher Server container with the additional environment variables.

```bash
$ sudo docker run -d --restart=always --name=rancher-server \
-e "CATTLE_API_ALLOW_CLIENT_OVERRIDE=true" \
-e "CATTLE_HOST_API_PROXY_SCHEME=wss" rancher/server
```

> **Note:** We are assuming that you will run your proxy in a container. If you are going to run a proxy from the host, you will need to expose port 8080 by adding `-p 8080:8080` to the command. 

If you are converting an existing Rancher instance configured with a data volume or external DB, stop and remove the existing Rancher Server container. Launch the new container with `--volumes-from=<data_container>` or [external DB settings]({{site.baseurl}}/docs/installing-rancher/installing-server/#external-db). 

## Nginx Configuration

We are working on making a simpler configuration and allow different SSL termination options.

Here is the minimum NGINX configuration that will need to be configured. You should customize your configuration to meet best practices. 

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

**Important Setting Notes:**

* Please be sure to disable NGINX `proxy-buffering`. If this setting is enable, the hosts will ping back to the server and this forces the agent to reconnect. 
* `X-API-request-url` header must be configured. This header is how the API schema will show the correct information. We are working to support standard `X-Forwarded-*` headers. 

## Settings

After Rancher is launched with these settings, the UI will be up and you can find it at this URL: 

`https://<rancher_server_ip>/v1/settings/api.host`

Click on **Edit**. Set the value equal to a space " " and **Save**.







