FROM ubuntu:17.10
RUN apt-get update && apt-get install -y git curl ca-certificates unzip xz-utils && \
    useradd rancher && \
    mkdir -p /var/lib/rancher/etcd /var/lib/cattle && \
    chown -R rancher /var/lib/rancher /var/lib/cattle /usr/local/bin
RUN curl -sLf https://github.com/rancher/machine-package/releases/download/v0.13.0-rancher4-1/docker-machine.tar.gz | tar xvzf - -C /usr/bin && \
    curl -sLf https://github.com/rancher/helm/releases/download/v2.8.0-rancher2/helm > /usr/bin/helm && \
    curl -sLf https://github.com/rancher/helm/releases/download/v2.8.0-rancher2/tiller > /usr/bin/tiller && \
    chmod +x /usr/bin/tiller /usr/bin/helm
RUN mkdir /root/.kube && \
    ln -s /usr/bin/rancher /usr/bin/kubectl && \
    ln -s /var/lib/rancher/kube_config_cluster.yml /root/.kube/config
WORKDIR /var/lib/rancher

ENV CATTLE_UI_PATH /usr/share/rancher/ui
ENV CATTLE_UI_VERSION 2.0.22
RUN mkdir -p /usr/share/rancher/ui && \
    cd /usr/share/rancher/ui && \
    curl -sL https://releases.rancher.com/ui/${CATTLE_UI_VERSION}.tar.gz | tar xvzf - --strip-components=1 && \
    cd /var/lib/rancher

ARG VERSION=dev
ENV CATTLE_SERVER_VERSION ${VERSION}
COPY rancher /usr/bin/

ENV CATTLE_AGENT_IMAGE rancher/agent:v2.0.3-rc3
ENV CATTLE_SERVER_IMAGE rancher/server
VOLUME /var/lib/rancher
ENTRYPOINT ["rancher", "--http-listen-port=80", "--https-listen-port=443"]
