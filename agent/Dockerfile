FROM rancher/agent-base:v0.3.0
COPY register.py resolve_url.py run.sh /
ENTRYPOINT ["/run.sh"]
LABEL "io.rancher.container.system"="rancher-agent"
ENV RANCHER_AGENT_IMAGE rancher/agent:v1.2.0-rc1
