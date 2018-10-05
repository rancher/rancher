FROM ubuntu:18.04
RUN apt-get update && apt-get install -y git curl ca-certificates unzip xz-utils && \
    useradd rancher && \
    mkdir -p /var/lib/rancher/etcd /var/lib/cattle && \
    chown -R rancher /var/lib/rancher /var/lib/cattle /usr/local/bin
RUN mkdir /root/.kube && \
    ln -s /usr/bin/rancher /usr/bin/kubectl && \
    ln -s /var/lib/rancher/kube_config_cluster.yml /root/.kube/config && \
    ln -s /usr/bin/rancher /usr/bin/reset-password && \
    ln -s /usr/bin/rancher /usr/bin/ensure-default-admin
WORKDIR /var/lib/rancher

ENV CATTLE_HELM_VERSION v2.10.0-rancher4
ENV CATTLE_MACHINE_VERSION v0.15.0-rancher1-1
ENV LOGLEVEL_VERSION v0.1
ENV TINI_VERSION v0.18.0
ENV TELEMETRY_VERSION v0.5.1

RUN curl -sLf https://github.com/rancher/machine-package/releases/download/${CATTLE_MACHINE_VERSION}/docker-machine.tar.gz | tar xvzf - -C /usr/bin && \
    curl -sLf https://github.com/rancher/loglevel/releases/download/${LOGLEVEL_VERSION}/loglevel-amd64-${LOGLEVEL_VERSION}.tar.gz | tar xvzf - -C /usr/bin

RUN curl -sLf https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini > /usr/bin/tini && \
    curl -sLf https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/helm > /usr/bin/helm && \
    curl -sLf https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/tiller > /usr/bin/tiller && \
    curl -sLf https://github.com/rancher/telemetry/releases/download/${TELEMETRY_VERSION}/telemetry > /usr/bin/telemetry && \
    chmod +x /usr/bin/helm /usr/bin/tini /usr/bin/telemetry /usr/bin/tiller

ENV CATTLE_UI_PATH /usr/share/rancher/ui
ENV CATTLE_UI_VERSION 2.0.77
ENV CATTLE_CLI_VERSION V2.0.5

RUN mkdir -p /var/log/auditlog
ENV AUDIT_LOG_PATH /var/log/auditlog/rancher-api-audit.log
ENV AUDIT_LOG_MAXAGE 10
ENV AUDIT_LOG_MAXBACKUP 10
ENV AUDIT_LOG_MAXSIZE 100
ENV AUDIT_LEVEL 0
VOLUME /var/log/auditlog

RUN mkdir -p /usr/share/rancher/ui && \
    cd /usr/share/rancher/ui && \
    curl -sL https://releases.rancher.com/ui/${CATTLE_UI_VERSION}.tar.gz | tar xvzf - --strip-components=1 && \
    cd /var/lib/rancher

ENV CATTLE_CLI_URL_DARWIN  https://releases.rancher.com/cli/${CATTLE_CLI_VERSION}/rancher-darwin-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_LINUX   https://releases.rancher.com/cli/${CATTLE_CLI_VERSION}/rancher-linux-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_WINDOWS https://releases.rancher.com/cli/${CATTLE_CLI_VERSION}/rancher-windows-386-${CATTLE_CLI_VERSION}.zip

ARG VERSION=dev
ENV CATTLE_SERVER_VERSION ${VERSION}
COPY entrypoint.sh rancher /usr/bin/
RUN chmod +x /usr/bin/entrypoint.sh

ENV CATTLE_AGENT_IMAGE rancher/rancher-agent:${VERSION}
ENV CATTLE_WINDOWS_AGENT_IMAGE rancher/rancher-agent:${VERSION}-nanoserver-1803
ENV CATTLE_SERVER_IMAGE rancher/rancher

ENV SSL_CERT_DIR /etc/rancher/ssl
VOLUME /var/lib/rancher

ENTRYPOINT ["entrypoint.sh"]
