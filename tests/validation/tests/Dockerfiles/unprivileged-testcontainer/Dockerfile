FROM nginxinc/nginx-unprivileged as base
MAINTAINER Max Ross https://github.com/rancher-max

USER root
RUN apt-get update

RUN apt-get install -y wget
RUN apt-get install -y curl
RUN apt-get install -y iptables
RUN apt-get install -y dnsutils
RUN apt-get install -y iputils-ping

COPY ./run.sh /scripts/run.sh
RUN chmod 777 /scripts/run.sh
RUN chmod 777 /usr/share/nginx/html

USER 101

ENTRYPOINT [ "/scripts/run.sh" ]
CMD ["nginx", "-g", "daemon off;"]
