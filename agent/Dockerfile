FROM ubuntu:16.04
RUN apt-get update && \
        apt-get install --no-install-recommends -y \
        arptables \
        bridge-utils \
        ca-certificates \
        curl \
        iptables \
        iproute2 \
        conntrack \
        python2.7 \
        jq && \
    curl -s -L https://get.docker.com/builds/Linux/x86_64/docker-1.6.0 > /usr/bin/docker; chmod +x /usr/bin/docker && \
    mkdir -p /var/lib/cattle /var/lib/rancher && \
    ln -s /usr/bin/python2.7 /usr/bin/python && \
    curl -s https://bootstrap.pypa.io/get-pip.py | python2.7 && \
    pip install cattle
RUN curl -sL https://github.com/rancher/runc/releases/download/share-mnt-v0.2.1/share-mnt > /usr/bin/share-mnt && \
    chmod +x /usr/bin/share-mnt
RUN curl -sL https://github.com/rancher/weave/releases/download/r-v0.0.4/r > /usr/bin/r && \
    chmod +x /usr/bin/r
COPY register.py resolve_url.py run.sh /
ENTRYPOINT ["/run.sh"]
LABEL "io.rancher.container.system"="rancher-agent"
ENV HOST_DOCKER_SOCK /var/run/docker.sock
ENV RANCHER_AGENT_IMAGE rancher/agent:v1.1.0-rc14
