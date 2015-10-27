FROM ubuntu:14.04.3

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-7-jre-headless \
                    curl \
                    mysql-server \
                    xz-utils \
                    keychain \
                    unzip \
                    git

ENV CATTLE_HOME /var/lib/cattle
ENV DEFAULT_CATTLE_API_UI_INDEX //releases.rancher.com/ui/0.60.0
ENV CATTLE_DB_CATTLE_DATABASE mysql
ENV CATTLE_USE_LOCAL_ARTIFACTS true
ADD artifacts /usr/share/cattle

ADD service /service
ENV S6_SERVICE_DIR /service

COPY target/*static.tar.gz /s6-statics/
RUN cd / && for i in $(ls /s6-statics/*static.tar.gz);do tar -zxvf $i;done

VOLUME ["/var/lib/cattle"]
VOLUME ["/var/lib/mysql"]
VOLUME ["/var/log/mysql"]

EXPOSE 8080
ENV CATTLE_HOST_API_PROXY_MODE embedded
ENV CATTLE_RANCHER_SERVER_IMAGE v0.43.0
ENV CATTLE_RANCHER_COMPOSE_VERSION v0.5.0
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_LINUX_URL   https://releases.rancher.com/compose/beta/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-linux-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_DARWIN_URL  https://releases.rancher.com/compose/beta/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-darwin-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_WINDOWS_URL https://releases.rancher.com/compose/beta/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-windows-386-${CATTLE_RANCHER_COMPOSE_VERSION}.zip

EXPOSE 3306
ENV CATTLE_CATTLE_VERSION v0.106.0
ADD https://github.com/rancherio/cattle/releases/download/${CATTLE_CATTLE_VERSION}/cattle.jar /usr/share/cattle/
RUN /usr/share/cattle/cattle.sh extract
RUN curl -sL https:${DEFAULT_CATTLE_API_UI_INDEX}.tar.gz | tar xvzf - -C /usr/share/cattle/war --strip-components=1
RUN /usr/share/cattle/install_cattle_binaries

ENV DEFAULT_CATTLE_MACHINE_EXECUTE true
ENV DEFAULT_CATTLE_COMPOSE_EXECUTOR_EXECUTE true
ENV DEFAULT_CATTLE_CATALOG_EXECUTE true

CMD ["/usr/bin/s6-svscan", "/service"]
