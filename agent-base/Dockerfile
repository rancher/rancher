FROM ubuntu:16.04
RUN apt-get update && \
        apt-get install --no-install-recommends -y \
        arptables \
        bridge-utils \
        ca-certificates \
        conntrack \
        curl \
        ethtool \
        iproute2 \
        iptables \
        iputils-ping \
        jq \
        kmod \
        openssl \
        psmisc \
        python2.7 \
        tcpdump \
        vim-tiny && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /var/lib/cattle /var/lib/rancher && \
    ln -s /usr/bin/python2.7 /usr/bin/python && \
    curl -s https://bootstrap.pypa.io/get-pip.py | python2.7 && \
    pip install cattle && \
    curl -sL https://github.com/rancher/runc/releases/download/share-mnt-v0.2.1/share-mnt > /usr/bin/share-mnt && \
    chmod +x /usr/bin/share-mnt && \
    curl -sL https://github.com/rancher/weave/releases/download/r-v0.0.4/r > /usr/bin/r && \
    chmod +x /usr/bin/r && \
    curl -sLf https://get.docker.com/builds/Linux/x86_64/docker-1.10.3 > /usr/bin/docker && \
    chmod +x /usr/bin/docker && \
    rm /var/run && \
    mkdir /var/run
