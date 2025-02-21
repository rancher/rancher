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
FROM final AS base

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
ARG CHART_DEFAULT_BRANCH=dev-v2.11
ARG PARTNER_CHART_DEFAULT_BRANCH=main
ARG RKE2_CHART_DEFAULT_BRANCH=main
# kontainer-driver-metadata branch to be set for specific branch other than dev/master, logic at rancher/rancher/pkg/settings/setting.go
ARG CATTLE_KDM_BRANCH=dev-v2.11
ARG VERSION=${VERSION}

ENV CATTLE_CHART_DEFAULT_BRANCH=$CHART_DEFAULT_BRANCH
ENV CATTLE_PARTNER_CHART_DEFAULT_BRANCH=$PARTNER_CHART_DEFAULT_BRANCH
ENV CATTLE_RKE2_CHART_DEFAULT_BRANCH=$RKE2_CHART_DEFAULT_BRANCH
ENV CATTLE_MACHINE_VERSION v0.15.0-rancher125
ENV CATTLE_K3S_VERSION v1.32.1+k3s1
ENV CATTLE_MACHINE_PROVISION_IMAGE rancher/machine:${CATTLE_MACHINE_VERSION}
ENV CATTLE_ETCD_VERSION v3.5.16
ENV TINI_VERSION v0.18.0
ENV DOCKER_MACHINE_LINODE_VERSION v0.1.12
ENV LINODE_UI_DRIVER_VERSION v0.7.0
# make sure the version number is consistent with the one at Line 100 of pkg/data/management/machinedriver_data.go
ENV DOCKER_MACHINE_HARVESTER_VERSION v1.0.0
ENV CATTLE_KDM_BRANCH ${CATTLE_KDM_BRANCH}
ENV CATTLE_WINS_AGENT_VERSION v0.5.0-rc.2
ENV CATTLE_WINS_AGENT_INSTALL_SCRIPT https://raw.githubusercontent.com/rancher/wins/${CATTLE_WINS_AGENT_VERSION}/install.ps1
ENV CATTLE_WINS_AGENT_UNINSTALL_SCRIPT https://raw.githubusercontent.com/rancher/wins/${CATTLE_WINS_AGENT_VERSION}/uninstall.ps1
ENV CATTLE_WINS_AGENT_UPGRADE_IMAGE rancher/wins:${CATTLE_WINS_AGENT_VERSION}
ENV CATTLE_CSI_PROXY_AGENT_VERSION v1.1.3
# make sure the CATTLE_SYSTEM_AGENT_VERSION is consistent with tests/v2/codecoverage/package/Dockerfile
ENV CATTLE_SYSTEM_AGENT_VERSION v0.3.11
ENV CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX https://github.com/rancher/system-agent/releases/download
ENV CATTLE_SYSTEM_AGENT_UPGRADE_IMAGE rancher/system-agent:${CATTLE_SYSTEM_AGENT_VERSION}-suc
ENV CATTLE_SYSTEM_AGENT_INSTALLER_IMAGE rancher/system-agent-installer-
# make sure the ENV CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT is consistent with pkg/settings/setting.go to utlize the local version of install script downloaded during build/package
ENV CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT ${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/install.sh
ENV CATTLE_SYSTEM_AGENT_UNINSTALL_SCRIPT ${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/system-agent-uninstall.sh
ENV CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION 105.0.0

# AKS,EKS,GKE Operator charts versions
ARG CATTLE_AKS_OPERATOR_VERSION
ENV CATTLE_AKS_OPERATOR_VERSION=$CATTLE_AKS_OPERATOR_VERSION
ARG CATTLE_EKS_OPERATOR_VERSION
ENV CATTLE_EKS_OPERATOR_VERSION=$CATTLE_EKS_OPERATOR_VERSION
ARG CATTLE_GKE_OPERATOR_VERSION
ENV CATTLE_GKE_OPERATOR_VERSION=$CATTLE_GKE_OPERATOR_VERSION

# System charts minimal version
# Deprecated in favor of CATTLE_FLEET_VERSION.
ENV CATTLE_FLEET_MIN_VERSION=""
ARG CATTLE_FLEET_VERSION
ENV CATTLE_FLEET_VERSION=$CATTLE_FLEET_VERSION
ARG CATTLE_RANCHER_WEBHOOK_VERSION
ENV CATTLE_RANCHER_WEBHOOK_VERSION=$CATTLE_RANCHER_WEBHOOK_VERSION
ARG CATTLE_RANCHER_PROVISIONING_CAPI_VERSION
ENV CATTLE_RANCHER_PROVISIONING_CAPI_VERSION=$CATTLE_RANCHER_PROVISIONING_CAPI_VERSION
ARG CATTLE_CSP_ADAPTER_MIN_VERSION
ENV CATTLE_CSP_ADAPTER_MIN_VERSION=$CATTLE_CSP_ADAPTER_MIN_VERSION

RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    # Temporarily clone from our GitHub's main repo, to avoid unnecessary load
    # in git.rancher.io
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    # Charts need to be copied into the sha256 value of git url computed in https://github.com/rancher/rancher/blob/5ebda9ac23c06e9647b586ec38aa51cc9ff9b031/pkg/catalogv2/git/download.go#L102 to create a unique folder per url
    git clone -b $CATTLE_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/charts /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721/ && \
    git clone -b $CATTLE_PARTNER_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/partner-charts /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 && \
    git clone -b $CATTLE_RKE2_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/rke2-charts /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9 && \
    # Revert the previous change in git.rancher.io from .gitconfig
    rm "${HOME}/.gitconfig"

RUN curl -sLf https://github.com/rancher/machine/releases/download/${CATTLE_MACHINE_VERSION}/rancher-machine-${ARCH}.tar.gz | tar xvzf - -C /usr/bin && \
    chown root:root /usr/bin/rancher-machine && \
    curl -LO https://github.com/linode/docker-machine-driver-linode/releases/download/${DOCKER_MACHINE_LINODE_VERSION}/docker-machine-driver-linode_linux-${ARCH}.zip && \
    unzip docker-machine-driver-linode_linux-${ARCH}.zip -d /opt/drivers/management-state/bin && \
    mkdir -p /usr/share/rancher/ui/assets/ && \
    ln -s /opt/drivers/management-state/bin/docker-machine-driver-linode /usr/share/rancher/ui/assets/ && \
    rm docker-machine-driver-linode_linux-${ARCH}.zip

RUN curl -LO https://github.com/harvester/docker-machine-driver-harvester/releases/download/${DOCKER_MACHINE_HARVESTER_VERSION}/docker-machine-driver-harvester-${ARCH}.tar.gz && \
    tar -xf docker-machine-driver-harvester-${ARCH}.tar.gz -C /opt/drivers/management-state/bin && \
    ln -s /opt/drivers/management-state/bin/docker-machine-driver-harvester /usr/share/rancher/ui/assets/ && \
    rm docker-machine-driver-harvester-${ARCH}.tar.gz

ENV TINI_URL_amd64=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini \
    TINI_URL_arm64=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-arm64 \
    TINI_URL_s390x=https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-s390x \
    TINI_URL=TINI_URL_${ARCH}

ENV ETCD_URL=https://github.com/etcd-io/etcd/releases/download/${CATTLE_ETCD_VERSION}/etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}.tar.gz

# Set up K3s: copy the necessary binaries from the K3s image.
COPY --from=rancher/k3s:v1.32.1-k3s1 \
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
    chmod +x /usr/bin/tini && \
    mkdir -p /var/lib/rancher-data/driver-metadata

ENV CATTLE_UI_VERSION 2.11.0-alpha5
ENV CATTLE_DASHBOARD_UI_VERSION v2.11.0-alpha5
ENV CATTLE_CLI_VERSION v2.11.0-rc.1

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

RUN curl -sLf https://releases.rancher.com/kontainer-driver-metadata/${CATTLE_KDM_BRANCH}/data.json > /var/lib/rancher-data/driver-metadata/data.json

ENV ETCDCTL_API=3

ENV SSL_CERT_DIR /etc/rancher/ssl
ENV ETCD_UNSUPPORTED_ARCH ${ETCD_UNSUPPORTED_ARCH}


# Enable exporting of the k3s images as part of the build process.
FROM --platform=$BUILDPLATFORM registry.suse.com/bci/golang:1.23 AS images

ENV CATTLE_K3S_VERSION v1.32.1+k3s1

RUN zypper -n install libbtrfs-devel libgpgme-devel

WORKDIR /src
COPY hack/airgap/ /src/
RUN go mod download

RUN go build -tags k3s_export -o export-images ./...
RUN ./export-images -k3s-version ${CATTLE_K3S_VERSION} -output /src/k3s-airgap-images.tar


FROM --platform=$BUILDPLATFORM registry.suse.com/bci/golang:1.23 AS build
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

# Only invalidate cache if go.mod/go.sum changes.
COPY go.mod go.sum .
COPY pkg/apis/go.mod pkg/apis/go.sum pkg/apis/
COPY pkg/client/go.mod pkg/client/go.sum pkg/client/
RUN go mod download

COPY pkg/ pkg/
COPY main.go .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" -o rancher

FROM base

ARG VERSION=${VERSION}
ENV CATTLE_SERVER_VERSION ${VERSION}
ENV CATTLE_AGENT_IMAGE ${IMAGE_REPO}/rancher-agent:${VERSION}
ENV CATTLE_SERVER_IMAGE ${IMAGE_REPO}/rancher

COPY --chown=root:root --chmod=0755 \
    --from=build /app/rancher /usr/bin/
COPY --chown=root:root --chmod=0755 \
    --from=images /src/k3s-airgap-images.tar /var/lib/rancher/k3s/agent/images/

COPY --chown=root:root --chmod=0755 \
    package/loglevel \
    package/entrypoint.sh \
    package/jailer.sh /usr/bin/

VOLUME /var/lib/rancher
VOLUME /var/lib/kubelet
VOLUME /var/lib/cni
VOLUME /var/log

LABEL "io.artifacthub.package.logo-url"="https://raw.githubusercontent.com/rancher/ui/master/public/assets/images/logos/welcome-cow.svg" \
    "io.artifacthub.package.readme-url"="https://raw.githubusercontent.com/rancher/rancher/${VERSION}/README.md" \
    "org.opencontainers.image.description"="Rancher Manager: complete container management platform." \
    "org.opencontainers.image.title"="Rancher Manager: complete container management platform." \
    "org.opencontainers.image.source"="https://github.com/rancher/rancher" \
    "org.opencontainers.image.version"=${VERSION} \
    "org.opensuse.reference"=rancher/rancher:${VERSION}

ENTRYPOINT ["entrypoint.sh"]
