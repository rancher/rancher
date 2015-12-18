FROM ubuntu:14.04.2
RUN apt-get update && \
     apt-get install --no-install-recommends -y \
     arptables \
     bridge-utils \
     ca-certificates \
     curl \
     iptables \
     libaio1 \
     python-eventlet \
     python-minimal
RUN curl -s https://bootstrap.pypa.io/get-pip.py > get-pip.py && python get-pip.py && rm get-pip.py
RUN pip install cattle docker-py 
RUN apt-get update && apt-get install -y --no-install-recommends libssl-dev python-dev libffi-dev gcc &&\
    pip install --upgrade requests[security]==2.7.0 &&\
    apt-get remove -y --purge python-dev libffi-dev libssl-dev gcc && apt-get autoremove -y
RUN curl -s http://stedolan.github.io/jq/download/linux64/jq > /usr/bin/jq; chmod +x /usr/bin/jq
RUN curl -s -L https://get.docker.com/builds/Linux/x86_64/docker-1.6.0 > /usr/bin/docker; chmod +x /usr/bin/docker
RUN curl -s -L https://github.com/rancher/thin-provisioning-tools/releases/download/rancher-v0.1/pdata_tools > /usr/bin/pdata_tools; chmod +x /usr/bin/pdata_tools
RUN apt-get update && apt-get install -y --no-install-recommends conntrack
RUN mkdir -p /var/lib/cattle /var/lib/rancher
COPY register.py resolve_url.py agent.sh run.sh /
ENTRYPOINT ["/run.sh"]
LABEL "io.rancher.container.system"="rancher-agent"
ENV RANCHER_AGENT_IMAGE rancher/agent:v0.8.2
