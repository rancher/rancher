FROM --platform=$BUILDPLATFORM registry.suse.com/bci/golang:1.22 as build
ARG VERSION
ARG COMMIT
ARG RKE_VERSION
ARG CGO_ENABLED=0
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static"
ARG DEFAULT_VALUES="{\"rke-version\":\"${RKE_VERSION}\"}"
ARG LDFLAGS="-X github.com/rancher/rancher/pkg/version.Version=${VERSION} -X github.com/rancher/rancher/pkg/version.GitCommit=${COMMIT} -X github.com/rancher/rancher/pkg/settings.InjectDefaults=${DEFAULT_VALUES} ${LINKFLAGS}"
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN go mod download
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" -o rancher

FROM registry.suse.com/bci/bci-micro:15.6 AS final

# Temporary build stage image
FROM registry.suse.com/bci/bci-base:15.6 AS builder

# Install system packages using builder image that has zypper
COPY --from=final / /chroot/

# Install some packages with zypper in the chroot of the final micro image
RUN zypper refresh && \
    zypper --installroot /chroot -n in --no-recommends \
    git-core curl util-linux ca-certificates ca-certificates-mozilla unzip xz gzip sed tar shadow gawk vim-small \
    netcat-openbsd mkisofs openssh-clients openssl patterns-base-fips && \
    zypper --installroot /chroot clean -a && \
    rm -rf /chroot/var/cache/zypp/* /chroot/var/log/zypp/* /chroot/tmp/* /chroot/var/tmp/* /chroot/usr/share/doc/packages/*

# Main stage using bci-micro as the base image
FROM final

# Copy binaries and configuration files from builder to micro
COPY --from=builder /chroot/ /

# Test that some of the dependency binaries were copied
# and are working on the target image.
RUN /usr/bin/unshare --version && \
    /usr/bin/mount --version && \
    /usr/bin/umount --version && \
    /usr/bin/nsenter --version

RUN useradd rancher && \
    groupadd jail-accessors && \
    usermod -aG jail-accessors rancher && \
    mkdir -p /var/lib/rancher /var/lib/cattle /opt/jail /opt/drivers/management-state/bin && \
    chgrp jail-accessors /var/lib/rancher /var/lib/cattle /usr/local/bin && \
    chmod 750            /var/lib/rancher /var/lib/cattle /usr/local/bin && \
    chmod g+s            /var/lib/rancher /var/lib/cattle /usr/local/bin

RUN mkdir /root/.kube && \
    ln -s /etc/rancher/k3s/k3s.yaml /root/.kube/k3s.yaml  && \
    ln -s /etc/rancher/k3s/k3s.yaml /root/.kube/config && \
    ln -s /usr/bin/rancher /usr/bin/reset-password && \
    ln -s /usr/bin/rancher /usr/bin/ensure-default-admin
WORKDIR /var/lib/rancher

ARG ARCH=amd64
ARG ETCD_UNSUPPORTED_ARCH
ARG IMAGE_REPO=rancher
ARG SYSTEM_CHART_DEFAULT_BRANCH=dev-v2.8
ARG CHART_DEFAULT_BRANCH=dev-v2.8
ARG PARTNER_CHART_DEFAULT_BRANCH=main
ARG RKE2_CHART_DEFAULT_BRANCH=main
# kontainer-driver-metadata branch to be set for specific branch other than dev/master, logic at rancher/rancher/pkg/settings/setting.go
ARG CATTLE_KDM_BRANCH=release-v2.8
ARG VERSION=${VERSION}

ENV CATTLE_SYSTEM_CHART_DEFAULT_BRANCH=$SYSTEM_CHART_DEFAULT_BRANCH
ENV CATTLE_CHART_DEFAULT_BRANCH=$CHART_DEFAULT_BRANCH
ENV CATTLE_PARTNER_CHART_DEFAULT_BRANCH=$PARTNER_CHART_DEFAULT_BRANCH
ENV CATTLE_RKE2_CHART_DEFAULT_BRANCH=$RKE2_CHART_DEFAULT_BRANCH
ENV CATTLE_HELM_VERSION v2.16.8-rancher2
# make sure the version number is consistent with the one at the 1) tests/v2/codecoverage/package/Dockerfile, 2) Dockerfile.dapper, 3) go.mod and 4) the setting MachineProvisionImage in pkg/settings/setting.go
ENV CATTLE_MACHINE_VERSION v0.15.0-rancher118
ENV CATTLE_K3S_VERSION v1.28.6+k3s2
ENV CATTLE_MACHINE_PROVISION_IMAGE rancher/machine:${CATTLE_MACHINE_VERSION}
ENV CATTLE_ETCD_VERSION v3.5.9
ENV LOGLEVEL_VERSION v0.1.6
ENV TINI_VERSION v0.18.0
ENV TELEMETRY_VERSION v0.6.2
ENV DOCKER_MACHINE_LINODE_VERSION v0.1.12
ENV LINODE_UI_DRIVER_VERSION v0.7.0
# make sure the version number is consistent with the one at Line 100 of pkg/data/management/machinedriver_data.go
ENV DOCKER_MACHINE_HARVESTER_VERSION v0.7.2
ENV CATTLE_KDM_BRANCH ${CATTLE_KDM_BRANCH}
ENV HELM_VERSION v3.13.3
ENV KUSTOMIZE_VERSION v5.0.1
ENV CATTLE_WINS_AGENT_VERSION v0.4.19
ENV CATTLE_WINS_AGENT_INSTALL_SCRIPT https://raw.githubusercontent.com/rancher/wins/${CATTLE_WINS_AGENT_VERSION}/install.ps1
ENV CATTLE_WINS_AGENT_UNINSTALL_SCRIPT https://raw.githubusercontent.com/rancher/wins/${CATTLE_WINS_AGENT_VERSION}/uninstall.ps1
ENV CATTLE_WINS_AGENT_UPGRADE_IMAGE rancher/wins:${CATTLE_WINS_AGENT_VERSION}
ENV CATTLE_CSI_PROXY_AGENT_VERSION v1.1.3
# make sure the CATTLE_SYSTEM_AGENT_VERSION is consistent with tests/v2/codecoverage/package/Dockerfile
ENV CATTLE_SYSTEM_AGENT_VERSION v0.3.10
ENV CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX https://github.com/rancher/system-agent/releases/download
ENV CATTLE_SYSTEM_AGENT_UPGRADE_IMAGE rancher/system-agent:${CATTLE_SYSTEM_AGENT_VERSION}-suc
ENV CATTLE_SYSTEM_AGENT_INSTALLER_IMAGE rancher/system-agent-installer-
# make sure the ENV CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT is consistent with pkg/settings/setting.go to utlize the local version of install script downloaded during build/package
ENV CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT ${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/install.sh
ENV CATTLE_SYSTEM_AGENT_UNINSTALL_SCRIPT ${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/system-agent-uninstall.sh
ENV CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION 103.0.2+up0.6.1

# System charts minimal version
# Deprecated in favor of CATTLE_FLEET_VERSION.
ENV CATTLE_FLEET_MIN_VERSION=""
ARG CATTLE_FLEET_VERSION
ENV CATTLE_FLEET_VERSION=$CATTLE_FLEET_VERSION
ARG CATTLE_RANCHER_WEBHOOK_VERSION
ENV CATTLE_RANCHER_WEBHOOK_VERSION=$CATTLE_RANCHER_WEBHOOK_VERSION
ARG CATTLE_CSP_ADAPTER_MIN_VERSION
ENV CATTLE_CSP_ADAPTER_MIN_VERSION=$CATTLE_CSP_ADAPTER_MIN_VERSION

RUN mkdir -p /var/lib/rancher-data/local-catalogs/system-library && \
    mkdir -p /var/lib/rancher-data/local-catalogs/library && \
    mkdir -p /var/lib/rancher-data/local-catalogs/helm3-library && \
    mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    git clone -b $CATTLE_SYSTEM_CHART_DEFAULT_BRANCH --depth 1 https://github.com/rancher/system-charts /var/lib/rancher-data/local-catalogs/system-library && \
    # Temporarily clone from our GitHub's main repo, to avoid unnecessary load
    # in git.rancher.io
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    # Charts need to be copied into the sha256 value of git url computed in https://github.com/rancher/rancher/blob/5ebda9ac23c06e9647b586ec38aa51cc9ff9b031/pkg/catalogv2/git/download.go#L102 to create a unique folder per url
    git clone -b $CATTLE_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/charts /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721/ && \
    git clone -b $CATTLE_PARTNER_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/partner-charts /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 && \
    git clone -b $CATTLE_RKE2_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/rke2-charts /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9 && \
    # Revert the previous change in git.rancher.io from .gitconfig
    rm "${HOME}/.gitconfig" && \
    git clone -b master --depth 1 https://github.com/rancher/charts /var/lib/rancher-data/local-catalogs/library && \
    git clone -b master --depth 1 https://github.com/rancher/helm3-charts /var/lib/rancher-data/local-catalogs/helm3-library

RUN curl -sLf https://github.com/rancher/machine/releases/download/${CATTLE_MACHINE_VERSION}/rancher-machine-${ARCH}.tar.gz | tar xvzf - -C /usr/bin && \
    chown root:root /usr/bin/rancher-machine && \
    curl -sLf https://github.com/rancher/loglevel/releases/download/${LOGLEVEL_VERSION}/loglevel-${ARCH}-${LOGLEVEL_VERSION}.tar.gz | tar xvzf - -C /usr/bin && \
    curl -LO https://github.com/linode/docker-machine-driver-linode/releases/download/${DOCKER_MACHINE_LINODE_VERSION}/docker-machine-driver-linode_linux-amd64.zip && \
    unzip docker-machine-driver-linode_linux-amd64.zip -d /opt/drivers/management-state/bin && \
    mkdir -p /usr/share/rancher/ui/assets/ && \
    cp /opt/drivers/management-state/bin/docker-machine-driver-linode /usr/share/rancher/ui/assets/ && \
    rm docker-machine-driver-linode_linux-amd64.zip

RUN curl -LO https://github.com/harvester/docker-machine-driver-harvester/releases/download/${DOCKER_MACHINE_HARVESTER_VERSION}/docker-machine-driver-harvester-${ARCH}.tar.gz && \
    tar -xf docker-machine-driver-harvester-${ARCH}.tar.gz -C /opt/drivers/management-state/bin && \
    cp /opt/drivers/management-state/bin/docker-machine-driver-harvester /usr/share/rancher/ui/assets/ && \
    rm docker-machine-driver-harvester-${ARCH}.tar.gz

ENV TINI_URL_amd64=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini \
    TINI_URL_arm64=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-arm64 \
    TINI_URL_s390x=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-s390x \
    TINI_URL=TINI_URL_${ARCH}

ENV HELM_URL_V2_amd64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-helm \
    HELM_URL_V2_arm64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-helm-arm64 \
    HELM_URL_V2_s390x=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-helm-s390x \
    HELM_URL_V2=HELM_URL_V2_${ARCH} \
    HELM_URL_V3=https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz \
    TILLER_URL_amd64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-tiller \
    TILLER_URL_arm64=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-tiller-arm64 \
    TILLER_URL_s390x=https://github.com/rancher/helm/releases/download/${CATTLE_HELM_VERSION}/rancher-tiller-s390x \
    TILLER_URL=TILLER_URL_${ARCH} \
    ETCD_URL=https://github.com/etcd-io/etcd/releases/download/${CATTLE_ETCD_VERSION}/etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}.tar.gz \
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
    chown root:root /usr/bin/helm_v3 /usr/bin/kustomize && \
    chmod +x /usr/bin/kustomize

# Set up K3s: copy the necessary binaries from the K3s image.
COPY --from=rancher/k3s:v1.28.6-k3s2 \
    /bin/blkid \
    /bin/bandwidth \
    /bin/cni \
    /bin/conntrack \
    /bin/containerd \
    /bin/containerd-shim-runc-v2 \
    /bin/ethtool \
    /bin/firewall \
    /bin/ip \
    /bin/ipset \
    /bin/k3s \
    /bin/losetup \
    /bin/pigz \
    /bin/runc \
    /bin/which \
    /bin/aux/xtables-legacy-multi \
    /usr/bin/

RUN ln -s /usr/bin/cni /usr/bin/bridge && \
    ln -s /usr/bin/cni /usr/bin/flannel && \
    ln -s /usr/bin/cni /usr/bin/host-local && \
    ln -s /usr/bin/cni /usr/bin/loopback && \
    ln -s /usr/bin/cni /usr/bin/portmap && \
    ln -s /usr/bin/k3s /usr/bin/crictl && \
    ln -s /usr/bin/k3s /usr/bin/ctr && \
    ln -s /usr/bin/k3s /usr/bin/k3s-agent && \
    ln -s /usr/bin/k3s /usr/bin/k3s-etcd-snapshot && \
    ln -s /usr/bin/k3s /usr/bin/k3s-server && \
    ln -s /usr/bin/k3s /usr/bin/kubectl && \
    ln -s /usr/bin/pigz /usr/bin/unpigz && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables-save && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables-restore && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables-translate && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables-save && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables-restore && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables-translate

RUN curl -sLf ${!TINI_URL} > /usr/bin/tini && \
    mkdir -p /var/lib/rancher/k3s/agent/images/ && \
    curl -sfL ${ETCD_URL} | tar xvzf - --strip-components=1 --no-same-owner -C /usr/bin/ etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}/etcdctl && \
    curl -sLf https://github.com/rancher/telemetry/releases/download/${TELEMETRY_VERSION}/telemetry-${ARCH} > /usr/bin/telemetry && \
    chmod +x /usr/bin/tini /usr/bin/telemetry && \
    mkdir -p /var/lib/rancher-data/driver-metadata


ENV CATTLE_UI_VERSION 2.8.14-alpha1
ENV CATTLE_DASHBOARD_UI_VERSION v2.8.14-alpha1
ENV CATTLE_CLI_VERSION v2.8.8


# Base UI brand used as a fallback env setting (not user facing) to indicate this is a non-prime install
ENV CATTLE_BASE_UI_BRAND=

# Please update the api-ui-version in pkg/settings/settings.go when updating the version here.
ENV CATTLE_API_UI_VERSION 1.1.11

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
    curl -sfL ${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-arm64 -O && \
    curl -sfL ${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-amd64 -O && \
    curl -sfL ${CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT} -o system-agent-install.sh && \
    curl -sfL ${CATTLE_SYSTEM_AGENT_UNINSTALL_SCRIPT} -o system-agent-uninstall.sh && \
    curl -sfL https://github.com/rancher/wins/releases/download/${CATTLE_WINS_AGENT_VERSION}/wins.exe -O && \
    curl -sfL https://acs-mirror.azureedge.net/csi-proxy/${CATTLE_CSI_PROXY_AGENT_VERSION}/binaries/csi-proxy-${CATTLE_CSI_PROXY_AGENT_VERSION}.tar.gz -O && \
    curl -sfL ${CATTLE_WINS_AGENT_INSTALL_SCRIPT} -o wins-agent-install.ps1 \
    curl -sfL ${CATTLE_WINS_AGENT_UNINSTALL_SCRIPT} -o wins-agent-uninstall.ps1

ENV CATTLE_CLI_URL_DARWIN  https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-darwin-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_LINUX   https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-linux-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_WINDOWS https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-windows-386-${CATTLE_CLI_VERSION}.zip

ENV CATTLE_SERVER_VERSION ${VERSION}

COPY --from=build /app/rancher /usr/bin/
COPY --from=build /app/package/entrypoint.sh /usr/bin/
COPY --from=build /app/package/kustomize.sh /usr/bin/
COPY --from=build app/package/jailer.sh /usr/bin/
COPY --from=build /app/k3s-airgap-images.tar /var/lib/rancher/k3s/agent/images/
RUN chmod +x /usr/bin/entrypoint.sh
RUN chmod +x /usr/bin/kustomize.sh

COPY --from=build /app/data.json /var/lib/rancher-data/driver-metadata/

ENV CATTLE_AGENT_IMAGE ${IMAGE_REPO}/rancher-agent:${VERSION}
ENV CATTLE_SERVER_IMAGE ${IMAGE_REPO}/rancher
ENV ETCDCTL_API=3

ENV SSL_CERT_DIR /etc/rancher/ssl
VOLUME /var/lib/rancher
VOLUME /var/lib/kubelet
VOLUME /var/lib/cni
VOLUME /var/log

ENV ETCD_UNSUPPORTED_ARCH ${ETCD_UNSUPPORTED_ARCH}

ENTRYPOINT ["entrypoint.sh"]
