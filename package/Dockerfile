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
VOLUME /var/lib/rancher
ENV ETCD_ARGS "--data-dir /var/lib/rancher/etcd"
USER rancher
WORKDIR /var/lib/rancher
COPY rancher /usr/bin/
ENTRYPOINT ["rancher", "--k8s-mode=exec"]
