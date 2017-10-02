FROM rancher/agent-base:v0.3.0
COPY register.py resolve_url.py run.sh /
ENTRYPOINT ["/run.sh"]
LABEL "io.rancher.container.system"="rancher-agent"
RUN curl -sL https://github.com/rancher/share-mnt/releases/download/v1.0.0/share-mnt.tar.gz | tar xvzf - -C /usr/bin
ENV RANCHER_AGENT_IMAGE rancher/agent:v1.2.7-rc1
