FROM ubuntu:17.10
RUN apt-get update && apt-get install -y git curl ca-certificates unzip xz-utils && \
    useradd rancher && \
    mkdir -p /var/lib/rancher/etcd /var/lib/cattle && \
    chown -R rancher /var/lib/rancher /var/lib/cattle /usr/local/bin
RUN curl -sLf https://github.com/rancher/machine-package/releases/download/v0.13.0-rancher3/docker-machine.tar.gz | tar xvzf - -C /usr/bin && \
    curl -sLf https://storage.googleapis.com/kubernetes-release/release/v1.8.6/bin/linux/amd64/kubectl > /usr/bin/kubectl && \
    curl -sLf https://github.com/rancher/helm/releases/download/v2.8.0-rancher1/helm > /usr/bin/helm && \
    curl -sLf https://github.com/rancher/helm/releases/download/v2.8.0-rancher1/tiller > /usr/bin/tiller && \
    chmod +x /usr/bin/kubectl /usr/bin/tiller /usr/bin/helm
ENV ETCD_ARGS "--data-dir /var/lib/rancher/etcd"
WORKDIR /var/lib/rancher

ENV CATTLE_UI_VERSION 2.0.15
RUN mkdir -p /usr/share/rancher/ui && \
    cd /usr/share/rancher/ui && \
    curl -sL https://releases.rancher.com/ui/${CATTLE_UI_VERSION}.tar.gz | tar xvzf - --strip-components=1 && \
    cd /var/lib/rancher && \
    ln -s /usr/share/rancher/ui ui

ARG VERSION=dev
ENV CATTLE_SERVER_VERSION ${VERSION}
COPY rancher /usr/bin/

ENV CATTLE_SERVER_IMAGE rancher/server
VOLUME /var/lib/rancher
ENTRYPOINT ["rancher", "--k8s-mode=exec", "--http-listen-port=80", "--https-listen-port=443"]
