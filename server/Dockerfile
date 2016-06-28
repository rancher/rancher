FROM ubuntu:14.04.4

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-7-jre-headless \
                    curl \
                    mysql-server \
                    xz-utils \
                    keychain \
                    unzip \
                    openssh-client \
                    iptables \
                    git \
                    redis-server \
                    zookeeper \
                    spiped

ADD https://github.com/cloudnautique/giddyup/releases/download/v0.10.0/giddyup /usr/bin/
ADD https://github.com/rancher/cluster-manager/releases/download/v0.1.5/cluster-manager /usr/bin/
RUN chmod +x /usr/bin/giddyup /usr/bin/cluster-manager
COPY bin/* /usr/bin/

ENV CATTLE_HOME /var/lib/cattle
ENV DEFAULT_CATTLE_API_UI_INDEX //releases.rancher.com/ui/1.1.11
ENV CATTLE_DB_CATTLE_DATABASE mysql
ENV CATTLE_USE_LOCAL_ARTIFACTS true
ENV no_proxy localhost,127.0.0.1,localaddress,.localdomain.com
ADD artifacts /usr/share/cattle

ADD service /service
ENV S6_SERVICE_DIR /service

COPY target/*static.tar.gz /s6-statics/

EXPOSE 8080
ENV CATTLE_HOST_API_PROXY_MODE embedded
ENV CATTLE_RANCHER_SERVER_IMAGE v1.1.0-rc3
ENV CATTLE_RANCHER_COMPOSE_VERSION v0.8.5
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_LINUX_URL   https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-linux-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_DARWIN_URL  https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-darwin-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_WINDOWS_URL https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-windows-386-${CATTLE_RANCHER_COMPOSE_VERSION}.zip
ENV DEFAULT_CATTLE_CATALOG_URL="library=https://github.com/rancher/rancher-catalog.git,community=https://github.com/rancher/community-catalog.git"

EXPOSE 3306
ENV CATTLE_CATTLE_VERSION v0.165.2
ADD https://github.com/rancherio/cattle/releases/download/${CATTLE_CATTLE_VERSION}/cattle.jar /usr/share/cattle/

RUN cd / && for i in $(ls /s6-statics/*static.tar.gz);do tar -zxvf $i;done && rm -rf /s6-statics/*static.tar.gz && \
    mkdir -p $CATTLE_HOME && \
    /usr/share/cattle/cattle.sh extract && \
    curl -sL https:${DEFAULT_CATTLE_API_UI_INDEX}.tar.gz | tar xvzf - -C /usr/share/cattle/war --strip-components=1 && \
    /usr/share/cattle/install_cattle_binaries && \
    cd $CATTLE_HOME && export IFS="," &&\
    for i in $DEFAULT_CATTLE_CATALOG_URL; do rancher-catalog-service -validate -catalogUrl=$i;done

VOLUME /var/lib/mysql /var/log/mysql /var/lib/cattle

ENV DEFAULT_CATTLE_MACHINE_EXECUTE true
ENV DEFAULT_CATTLE_COMPOSE_EXECUTOR_EXECUTE true
ENV DEFAULT_CATTLE_CATALOG_EXECUTE true

CMD ["/usr/bin/s6-svscan", "/service"]
