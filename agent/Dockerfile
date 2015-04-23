FROM ubuntu:14.04.2
RUN apt-get update && \
     apt-get install --no-install-recommends -y \
     arptables \
     bridge-utils \
     ca-certificates \
     curl \
     iptables \
     python-eventlet \
     python-minimal
RUN curl -s https://bootstrap.pypa.io/get-pip.py > get-pip.py && python get-pip.py && rm get-pip.py
RUN pip install cattle docker-py
RUN curl -s http://stedolan.github.io/jq/download/linux64/jq > /usr/bin/jq; chmod +x /usr/bin/jq
RUN curl -s https://get.docker.io/builds/Linux/x86_64/docker-1.5.0 > /usr/bin/docker; chmod +x /usr/bin/docker
RUN apt-get update && apt-get install -y --no-install-recommends nodejs
RUN mkdir -p /var/lib/cattle /var/lib/rancher
COPY register.py resolve_url.py agent.sh run.sh /
ENTRYPOINT ["/run.sh"]
ENV RANCHER_AGENT_IMAGE rancher/agent:v0.6.1
