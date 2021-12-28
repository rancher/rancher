FROM registry.suse.com/suse/sle15:15.3

RUN zypper -n install git-core curl ca-certificates unzip xz gzip sed tar shadow gawk vim && \
    zypper -n clean -a && rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/* && \
    useradd rancher && \
    mkdir -p /var/lib/rancher /var/lib/cattle /opt/jail /opt/drivers/management-state/bin && \
    chown -R rancher /var/lib/rancher /var/lib/cattle /usr/local/bin

RUN mkdir /root/.kube && \
    ln -s /etc/rancher/k3s/k3s.yaml /root/.kube/k3s.yaml  && \
    ln -s /etc/rancher/k3s/k3s.yaml /root/.kube/config && \
    ln -s /usr/bin/rancher /usr/bin/reset-password && \
    ln -s /usr/bin/rancher /usr/bin/ensure-default-admin
WORKDIR /var/lib/rancher

ARG ARCH=amd64
ARG IMAGE_REPO=rancher
ARG SYSTEM_CHART_DEFAULT_BRANCH=dev-v2.6
ARG CHART_DEFAULT_BRANCH=dev-v2.6
ARG PARTNER_CHART_DEFAULT_BRANCH=main
ARG RKE2_CHART_DEFAULT_BRANCH=main
# kontainer-driver-metadata branch to be set for specific branch other than dev/master, logic at rancher/rancher/pkg/settings/setting.go
ARG CATTLE_KDM_BRANCH=dev-v2.6

ENV CATTLE_SYSTEM_CHART_DEFAULT_BRANCH=$SYSTEM_CHART_DEFAULT_BRANCH
ENV CATTLE_CHART_DEFAULT_BRANCH=$CHART_DEFAULT_BRANCH
ENV CATTLE_PARTNER_CHART_DEFAULT_BRANCH=$PARTNER_CHART_DEFAULT_BRANCH
ENV CATTLE_RKE2_CHART_DEFAULT_BRANCH=$RKE2_CHART_DEFAULT_BRANCH
ENV CATTLE_HELM_VERSION v2.16.8-rancher1
ENV CATTLE_K3S_VERSION v1.21.7+k3s1
ENV CATTLE_MACHINE_VERSION v0.15.0-rancher73
ENV CATTLE_MACHINE_PROVISION_IMAGE rancher/machine:${CATTLE_MACHINE_VERSION}
ENV CATTLE_ETCD_VERSION v3.4.13
ENV LOGLEVEL_VERSION v0.1.3
ENV TINI_VERSION v0.18.0
ENV TELEMETRY_VERSION v0.5.16
ENV DOCKER_MACHINE_LINODE_VERSION v0.1.8
ENV LINODE_UI_DRIVER_VERSION v0.3.0
# make sure the version number is consistent with the one at Line 100 of pkg/data/management/machinedriver_data.go
ENV DOCKER_MACHINE_HARVESTER_VERSION v0.3.4
ENV CATTLE_KDM_BRANCH ${CATTLE_KDM_BRANCH}
ENV HELM_VERSION v3.7.1
ENV KUSTOMIZE_VERSION v3.10.0
ENV CATTLE_SYSTEM_AGENT_VERSION v0.1.1
ENV CATTLE_SYSTEM_AGENT_UPGRADE_IMAGE rancher/system-agent:${CATTLE_SYSTEM_AGENT_VERSION}-suc

# System charts minimal version
ENV CATTLE_FLEET_MIN_VERSION=100.0.2+up0.3.8
ENV CATTLE_RANCHER_WEBHOOK_MIN_VERSION=1.0.2+up0.2.2

RUN mkdir -p /var/lib/rancher-data/local-catalogs/system-library && \
    mkdir -p /var/lib/rancher-data/local-catalogs/library && \
    mkdir -p /var/lib/rancher-data/local-catalogs/helm3-library && \
    mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    git clone -b $CATTLE_SYSTEM_CHART_DEFAULT_BRANCH --depth 1 https://github.com/rancher/system-charts /var/lib/rancher-data/local-catalogs/system-library && \
    # Charts need to be copied into the sha256 value of git url computed in https://github.com/rancher/rancher/blob/5ebda9ac23c06e9647b586ec38aa51cc9ff9b031/pkg/catalogv2/git/download.go#L102 to create a unique folder per url
    git clone -b $CATTLE_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/charts /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721/ && \
    git clone -b $CATTLE_PARTNER_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/partner-charts /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 && \
    git clone -b $CATTLE_RKE2_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/rke2-charts /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9 && \
    git clone -b master --depth 1 https://github.com/rancher/charts /var/lib/rancher-data/local-catalogs/library && \
    git clone -b master --depth 1 https://github.com/rancher/helm3-charts /var/lib/rancher-data/local-catalogs/helm3-library

RUN curl -sLf https://github.com/rancher/machine/releases/download/${CATTLE_MACHINE_VERSION}/rancher-machine-${ARCH}.tar.gz | tar xvzf - -C /usr/bin && \
    curl -sLf https://github.com/rancher/loglevel/releases/download/${LOGLEVEL_VERSION}/loglevel-${ARCH}-${LOGLEVEL_VERSION}.tar.gz | tar xvzf - -C /usr/bin && \
    curl -LO https://github.com/linode/docker-machine-driver-linode/releases/download/${DOCKER_MACHINE_LINODE_VERSION}/docker-machine-driver-linode_linux-amd64.zip && \
    unzip docker-machine-driver-linode_linux-amd64.zip -d /opt/drivers/management-state/bin && \
    rm docker-machine-driver-linode_linux-amd64.zip

RUN curl -LO https://releases.rancher.com/harvester-node-driver/${DOCKER_MACHINE_HARVESTER_VERSION}/docker-machine-driver-harvester-amd64.tar.gz && \
    tar -xf docker-machine-driver-harvester-amd64.tar.gz -C /opt/drivers/management-state/bin && \
    rm docker-machine-driver-harvester-amd64.tar.gz

ENV TINI_URL_amd64=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini \
    TINI_URL_arm64=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-arm64 \
    TINI_URL=TINI_URL_${ARCH}

ENV HELM_URL_V2_amd64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-helm \
    HELM_URL_V2_arm64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-helm-arm64 \
    HELM_URL_V2=HELM_URL_V2_${ARCH} \
    HELM_URL_V3=https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz \
    TILLER_URL_amd64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-tiller \
    TILLER_URL_arm64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-tiller-arm64 \
    TILLER_URL=TILLER_URL_${ARCH} \
    ETCD_URL=https://github.com/etcd-io/etcd/releases/download/${CATTLE_ETCD_VERSION}/etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}.tar.gz \
    K3S_DOWNLOAD_URL_amd64=https://github.com/rancher/k3s/releases/download/${CATTLE_K3S_VERSION}/k3s \
    K3S_DOWNLOAD_URL_arm64=https://github.com/rancher/k3s/releases/download/${CATTLE_K3S_VERSION}/k3s-arm64 \
    K3S_DOWNLOAD_URL=K3S_DOWNLOAD_URL_${ARCH} \
    KUSTOMIZE_URL=https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_${ARCH}.tar.gz

RUN curl -sLf ${KUSTOMIZE_URL} | tar -xzf - -C /usr/bin

# set up helm 2
RUN curl -sLf ${!HELM_URL_V2} > /usr/bin/rancher-helm && \
    curl -sLf ${!TILLER_URL} > /usr/bin/rancher-tiller && \
    ln -s /usr/bin/rancher-helm /usr/bin/helm && \
    ln -s /usr/bin/rancher-tiller /usr/bin/tiller && \
    chmod +x /usr/bin/rancher-helm /usr/bin/rancher-tiller

# set up helm 3
RUN curl ${HELM_URL_V3} | tar xvzf - --strip-components=1 -C /usr/bin && \
    mv /usr/bin/helm /usr/bin/helm_v3 && \
    chmod +x /usr/bin/kustomize

RUN curl -sLf ${!TINI_URL} > /usr/bin/tini && \
    curl -sLf ${!K3S_DOWNLOAD_URL} > /usr/bin/k3s && \
    mkdir -p /var/lib/rancher/k3s/agent/images/ && \
    curl -sfL ${ETCD_URL} | tar xvzf - --strip-components=1 -C /usr/bin/ etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}/etcdctl && \
    curl -sLf https://github.com/rancher/telemetry/releases/download/${TELEMETRY_VERSION}/telemetry-${ARCH} > /usr/bin/telemetry && \
    curl -sLf https://storage.googleapis.com/kubernetes-release/release/${CATTLE_K3S_VERSION%+*}/bin/linux/${ARCH}/kubectl > /usr/bin/kubectl && \
    chmod +x /usr/bin/tini /usr/bin/telemetry /usr/bin/k3s /usr/bin/kubectl && \
    mkdir -p /var/lib/rancher-data/driver-metadata

ENV CATTLE_UI_VERSION 2.6.3
ENV CATTLE_DASHBOARD_UI_VERSION v2.6.3
ENV CATTLE_CLI_VERSION v2.6.0

# Please update the api-ui-version in pkg/settings/settings.go when updating the version here.
ENV CATTLE_API_UI_VERSION 1.1.9

RUN mkdir -p /var/log/auditlog
ENV AUDIT_LOG_PATH /var/log/auditlog/rancher-api-audit.log
ENV AUDIT_LOG_MAXAGE 10
ENV AUDIT_LOG_MAXBACKUP 10
ENV AUDIT_LOG_MAXSIZE 100
ENV AUDIT_LEVEL 0

RUN mkdir -p /usr/share/rancher/ui && \
    cd /usr/share/rancher/ui && \
    curl -sL https://releases.rancher.com/ui/${CATTLE_UI_VERSION}.tar.gz | tar xvzf - --strip-components=1 && \
    mkdir -p assets/rancher-ui-driver-linode && \
    cd assets/rancher-ui-driver-linode && \
    curl -O https://linode.github.io/rancher-ui-driver-linode/releases/${LINODE_UI_DRIVER_VERSION}/component.js && \
    curl -O https://linode.github.io/rancher-ui-driver-linode/releases/${LINODE_UI_DRIVER_VERSION}/component.css && \
    curl -O https://linode.github.io/rancher-ui-driver-linode/releases/${LINODE_UI_DRIVER_VERSION}/linode.svg && \
    mkdir -p /usr/share/rancher/ui/api-ui && \
    cd /usr/share/rancher/ui/api-ui && \
    curl -sL https://releases.rancher.com/api-ui/${CATTLE_API_UI_VERSION}.tar.gz | tar xvzf - --strip-components=1 && \
    mkdir -p /usr/share/rancher/ui-dashboard/dashboard && \
    cd /usr/share/rancher/ui-dashboard/dashboard && \
    curl -sL https://releases.rancher.com/dashboard/${CATTLE_DASHBOARD_UI_VERSION}.tar.gz | tar xvzf - --strip-components=2 && \
    ln -s dashboard/index.html ../index.html && \
    cd ../../ui/assets && \
    curl -sfL https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-arm -O && \
    curl -sfL https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-arm64 -O && \
    curl -sfL https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-amd64 -O && \
    curl -sfL https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/install.sh -o system-agent-install.sh && \
    curl -sfL https://raw.githubusercontent.com/rancher/rke2/master/windows/rke2-install.ps1 -o rke2-install.ps1

ENV CATTLE_CLI_URL_DARWIN  https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-darwin-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_LINUX   https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-linux-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_WINDOWS https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-windows-386-${CATTLE_CLI_VERSION}.zip

ARG VERSION=dev
ENV CATTLE_SERVER_VERSION ${VERSION}
COPY entrypoint.sh rancher /usr/bin/
COPY kustomize.sh /usr/bin/
COPY jailer.sh /usr/bin/
COPY k3s-airgap-images.tar /var/lib/rancher/k3s/agent/images/
RUN chmod +x /usr/bin/entrypoint.sh
RUN chmod +x /usr/bin/kustomize.sh

COPY data.json /var/lib/rancher-data/driver-metadata/

ENV CATTLE_AGENT_IMAGE ${IMAGE_REPO}/rancher-agent:${VERSION}
ENV CATTLE_SERVER_IMAGE ${IMAGE_REPO}/rancher
ENV ETCDCTL_API=3

ENV SSL_CERT_DIR /etc/rancher/ssl
VOLUME /var/lib/rancher
VOLUME /var/lib/kubelet
VOLUME /var/lib/cni
VOLUME /var/log

ENTRYPOINT ["entrypoint.sh"]
