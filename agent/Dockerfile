# Install nsenter move to ubuntu:14.04.1
FROM ibuildthecloud/ubuntu-core-base:14.04-1
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
RUN curl -s https://get.docker.io/builds/Linux/x86_64/docker-1.3.2 > /usr/bin/docker; chmod +x /usr/bin/docker
RUN apt-get update && apt-get install -y --no-install-recommends nodejs
COPY agent-env.sh /
COPY register.py /
COPY resolve_url.py /
COPY startup.sh /
ENTRYPOINT ["/startup.sh"]
ENV RANCHER_AGENT_IMAGE rancher/agent:v0.2.1
