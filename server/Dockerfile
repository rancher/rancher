FROM ubuntu:14.04.2

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-7-jre-headless \
                    curl \
                    mysql-server \
                    xz-utils \
                    keychain

ENV CATTLE_HOME /var/lib/cattle
ENV DEFAULT_CATTLE_API_UI_INDEX //releases.rancher.com/ui/0.25.0
ENV CATTLE_DB_CATTLE_DATABASE mysql
ADD artifacts /usr/share/cattle

ADD service /service
ENV S6_SERVICE_DIR /service

COPY target/*static.tar.gz /s6-statics/
RUN cd / && for i in $(ls /s6-statics/*static.tar.gz);do tar -zxvf $i;done

VOLUME ["/var/lib/cattle"]
VOLUME ["/var/lib/mysql"]

EXPOSE 8080
ENV CATTLE_RANCHER_SERVER_IMAGE v0.24.0-rc2
ENV CATTLE_RANCHER_COMPOSE_VERSION v0.1.1
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_LINUX_URL   https://github.com/rancherio/rancher-compose/releases/download/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose_linux-amd64
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_DARWIN_URL  https://github.com/rancherio/rancher-compose/releases/download/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose_darwin-amd64
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_WINDOWS_URL https://github.com/rancherio/rancher-compose/releases/download/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose_windows-386.exe

EXPOSE 3306
ENV CATTLE_CATTLE_VERSION v0.53.0
ADD https://github.com/rancherio/cattle/releases/download/${CATTLE_CATTLE_VERSION}/cattle.jar /usr/share/cattle/

ENV CATTLE_DOCKER_MACHINE_VERSION v0.3.0-dev-packet-rc2
ADD https://github.com/wlan0/machine/releases/download/${CATTLE_DOCKER_MACHINE_VERSION}/docker-machine /usr/bin/docker-machine
RUN chmod +x /usr/bin/docker-machine

ENV CATTLE_GO_MACHINE_SERVICE_VERSION v0.13.0
ADD https://github.com/rancherio/go-machine-service/releases/download/${CATTLE_GO_MACHINE_SERVICE_VERSION}/go-machine-service.tar.xz /tmp/
RUN tar xpvf /tmp/go-machine-service.tar.xz -C /usr/bin/ && \
    chmod +x /usr/bin/go-machine-service

ADD https://github.com/rancherio/websocket-proxy/releases/download/v0.1.1/websocket-proxy.tar.xz /tmp/
RUN tar xpvf /tmp/websocket-proxy.tar.xz -C /usr/bin/ && \
    chmod +x /usr/bin/websocket-proxy

ENV DEFAULT_CATTLE_MACHINE_EXECUTE true
CMD ["/usr/bin/s6-svscan", "/service"]
