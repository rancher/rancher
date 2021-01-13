#!/bin/bash

if [ -z "$CONTAINER_NAME" ]; then
    hostname > /usr/share/nginx/html/name.html
    hostname > /usr/share/nginx/html/service1.html
    hostname > /usr/share/nginx/html/service2.html
else
    echo ${CONTAINER_NAME} > /usr/share/nginx/html/name.html
    echo ${CONTAINER_NAME} > /usr/share/nginx/html/service1.html
    echo ${CONTAINER_NAME} > /usr/share/nginx/html/service2.html
fi
exec "$@"
