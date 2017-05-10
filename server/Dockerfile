FROM azul/zulu-openjdk:8u72

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
                    ca-certificates \
                    curl \
                    mysql-server \
                    xz-utils \
                    keychain \
                    unzip \
                    openssh-client \
                    iptables \
                    git \
    && rm -rf /var/lib/apt/lists/*

# In Zulu these two JAR files are built directly from OpenJDK source,
# and result in a local_policy JAR limited to 128-bit max cipher lengths, and a
# US_export_policy set no restriction (very large keys, ie. precise value is
# 2^32-1 or 0x7FFFFFFF).
#
# Make US export policy local policy
RUN cp /usr/lib/jvm/zulu-8-amd64/jre/lib/security/US_export_policy.jar /usr/lib/jvm/zulu-8-amd64/jre/lib/security/local_policy.jar

COPY bin/* /usr/bin/

ENV CATTLE_HOME /var/lib/cattle
ENV DEFAULT_CATTLE_API_UI_INDEX //releases.rancher.com/ui/1.6.3
ENV CATTLE_API_UI_URL //releases.rancher.com/api-ui/1.0.8
ENV CATTLE_DB_CATTLE_DATABASE mysql
ENV CATTLE_USE_LOCAL_ARTIFACTS true
ENV no_proxy localhost,127.0.0.1,localaddress,.localdomain.com
ADD artifacts /usr/share/cattle

ADD service /service
ENV S6_SERVICE_DIR /service

COPY target/*static.tar.gz /s6-statics/

EXPOSE 8080
ENV CATTLE_HOST_API_PROXY_MODE embedded
ENV CATTLE_RANCHER_SERVER_VERSION v1.6.1-rc3
ENV CATTLE_RANCHER_COMPOSE_VERSION v0.12.5
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_LINUX_URL   https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-linux-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_DARWIN_URL  https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-darwin-amd64-${CATTLE_RANCHER_COMPOSE_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_COMPOSE_WINDOWS_URL https://releases.rancher.com/compose/${CATTLE_RANCHER_COMPOSE_VERSION}/rancher-compose-windows-386-${CATTLE_RANCHER_COMPOSE_VERSION}.zip
ENV CATTLE_RANCHER_CLI_VERSION v0.6.0
ENV DEFAULT_CATTLE_RANCHER_CLI_LINUX_URL   https://releases.rancher.com/cli/${CATTLE_RANCHER_CLI_VERSION}/rancher-linux-amd64-${CATTLE_RANCHER_CLI_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_CLI_DARWIN_URL  https://releases.rancher.com/cli/${CATTLE_RANCHER_CLI_VERSION}/rancher-darwin-amd64-${CATTLE_RANCHER_CLI_VERSION}.tar.gz
ENV DEFAULT_CATTLE_RANCHER_CLI_WINDOWS_URL https://releases.rancher.com/cli/${CATTLE_RANCHER_CLI_VERSION}/rancher-windows-386-${CATTLE_RANCHER_CLI_VERSION}.zip
ENV DEFAULT_CATTLE_CATALOG_URL='{"catalogs":{"community":{"url":"https://git.rancher.io/community-catalog.git","branch":"master"},"library":{"url":"https://git.rancher.io/rancher-catalog.git","branch":"v1.6.1-rc1"}}}'


EXPOSE 3306
ENV CATTLE_CATTLE_VERSION v0.180.1
ADD https://github.com/rancherio/cattle/releases/download/${CATTLE_CATTLE_VERSION}/cattle.jar /usr/share/cattle/

RUN cd / && for i in $(ls /s6-statics/*static.tar.gz);do tar -C /usr -zxvf $i;done && rm -rf /s6-statics/*static.tar.gz && \
    mkdir -p $CATTLE_HOME && \
    /usr/share/cattle/cattle.sh extract && \
    curl -sL https:${DEFAULT_CATTLE_API_UI_INDEX}.tar.gz | tar xvzf - -C /usr/share/cattle/war --strip-components=1 && \
    mkdir -p /usr/share/cattle/war/api-ui && \
    curl -sL https:${CATTLE_API_UI_URL}.tar.gz | tar xvzf - -C /usr/share/cattle/war/api-ui --strip-components=1 && \
    /usr/share/cattle/install_cattle_binaries && \
    curl -sL https://github.com/prometheus/graphite_exporter/releases/download/v0.2.0/graphite_exporter-0.2.0.linux-amd64.tar.gz | tar -xzv -C /usr/bin/ --strip-components=1 && \
    cd $CATTLE_HOME && export IFS="," && \
    echo "$DEFAULT_CATTLE_CATALOG_URL" > repo.json && \
    cat repo.json && \
    rancher-catalog-service-sqlite --sqlite --validate --config repo.json && \
    for i in cache/*/*; do [ -d $i ] && touch $i/.nopurge; done

VOLUME /var/lib/mysql /var/log/mysql /var/lib/cattle

ENV DEFAULT_CATTLE_API_UI_JS_URL /api-ui/ui.min.js
ENV DEFAULT_CATTLE_API_UI_CSS_URL /api-ui/ui.min.css
ENV DEFAULT_CATTLE_MACHINE_EXECUTE true
ENV DEFAULT_CATTLE_COMPOSE_EXECUTOR_EXECUTE true
ENV DEFAULT_CATTLE_CATALOG_EXECUTE true
ENV DEFAULT_CATTLE_AUTH_SERVICE_EXECUTE true
ENV DEFAULT_CATTLE_WEBHOOK_SERVICE_EXECUTE true
ENV DEFAULT_CATTLE_SECRETS_API_EXECUTE true
ENV CATTLE_RANCHER_SERVER_IMAGE rancher/server

ENTRYPOINT ["/usr/bin/entry"]
CMD ["/usr/bin/s6-svscan", "/service"]
