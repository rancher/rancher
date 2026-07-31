ARG ARCH=amd64

ARG BUILD_WORKDIR=/app

ARG GODEP_APISERVER=scratch
ARG GODEP_APISERVER_PATH
ARG GODEP_LASSO=scratch
ARG GODEP_LASSO_PATH
ARG GODEP_NORMAN=scratch
ARG GODEP_NORMAN_PATH
ARG GODEP_REMOTEDIALER=scratch
ARG GODEP_REMOTEDIALER_PATH
ARG GODEP_SHEPHERD=scratch
ARG GODEP_SHEPHERD_PATH
ARG GODEP_STEVE=scratch
ARG GODEP_STEVE_PATH
ARG GODEP_WRANGLER=scratch
ARG GODEP_WRANGLER_PATH

ARG CHART_DEFAULT_BRANCH=dev-v2.15
ARG PARTNER_CHART_DEFAULT_BRANCH=main
ARG RKE2_CHART_DEFAULT_BRANCH=main
# kontainer-driver-metadata branch to be set for specific branch other than dev/master, logic at rancher/rancher/pkg/settings/setting.go
ARG CATTLE_KDM_BRANCH=dev-v2.15
ARG VERSION=${VERSION}

# TEMPORARY (CI cache benchmark): externally-provided go build/mod cache
# content, supplied as an extra buildx build-context (same pattern as the
# GODEP_* deps below). Defaults to "scratch" (empty) so normal builds that
# don't pass --build-context gocache=... behave exactly as before. When a
# real directory is passed via --build-arg GOCACHE_IN=gocache
# --build-context gocache=<path>, its content is COPY'd into /root/.cache
# as a plain layer -- NOT a RUN --mount=type=cache -- so it lives under
# BuildKit's normal (large) layer GC budget instead of the much smaller
# exec.cachemount budget that RUN --mount=type=cache is subject to on some
# runner fleets. See the CI test in pull-request.yml (cache-benchmark-*
# jobs) for how this is populated/extracted across builds.
ARG GOCACHE_IN=scratch

FROM --platform=$BUILDPLATFORM ${GODEP_APISERVER} AS godep-apiserver
FROM --platform=$BUILDPLATFORM ${GODEP_LASSO} AS godep-lasso
FROM --platform=$BUILDPLATFORM ${GODEP_NORMAN} AS godep-norman
FROM --platform=$BUILDPLATFORM ${GODEP_REMOTEDIALER} AS godep-remotedialer
FROM --platform=$BUILDPLATFORM ${GODEP_SHEPHERD} AS godep-shepherd
FROM --platform=$BUILDPLATFORM ${GODEP_STEVE} AS godep-steve
FROM --platform=$BUILDPLATFORM ${GODEP_WRANGLER} AS godep-wrangler
FROM --platform=$BUILDPLATFORM ${GOCACHE_IN} AS gocache-in

FROM registry.suse.com/bci/bci-micro:15.7 AS final
RUN : # No-op command to create an explicit layer - this fixes a weird buildkit/buildx bug on macos arm

# Temporary build stage image
FROM registry.suse.com/bci/bci-base:15.7 AS chroot-builder
# Install system packages using builder image that has zypper
COPY --from=final / /chroot/
# Install some packages with zypper in the chroot of the final micro image
RUN zypper refresh && \
    zypper --installroot /chroot -n in --no-recommends \
      catatonit curl util-linux ca-certificates ca-certificates-mozilla xz gzip tar gawk vim-small openssh-clients openssl patterns-base-fips


FROM chroot-builder AS chroot-builder-server
RUN zypper --installroot /chroot -n in --no-recommends \
      git-core unzip sed shadow netcat-openbsd mkisofs && \
    zypper --installroot /chroot clean -a && \
    rm -rf /chroot/var/cache/zypp/* /chroot/var/log/zypp/* /chroot/tmp/* /chroot/var/tmp/* /chroot/usr/share/doc/packages/*


FROM chroot-builder AS chroot-builder-agent
RUN zypper --installroot /chroot -n in --no-recommends \
      jq git-core hostname iproute2 less bash-completion bind-utils acl sysstat && \
    zypper --installroot /chroot clean -a && \
    rm -rf /chroot/var/cache/zypp/* /chroot/var/log/zypp/* /chroot/tmp/* /chroot/var/tmp/* /chroot/usr/share/doc/packages/*


FROM final AS builder
COPY --from=chroot-builder-server /chroot/ /

FROM --platform=$BUILDPLATFORM registry.suse.com/bci/golang:1.26.5 AS go-builder
ARG CGO_ENABLED=0
ENV CGO_ENABLED=$CGO_ENABLED
ENV GOMODCACHE=/root/.cache/go/modcache
ENV GOCACHE=/root/.cache/go/cache

FROM builder AS rancher-charts
ARG CHART_DEFAULT_BRANCH
RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    # Temporarily clone from our GitHub's main repo, to avoid unnecessary load in git.rancher.io
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    # Charts need to be copied into the sha256 value of git url computed in https://github.com/rancher/rancher/blob/5ebda9ac23c06e9647b586ec38aa51cc9ff9b031/pkg/catalogv2/git/download.go#L102 to create a unique folder per url
    git clone --no-checkout -b $CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/charts /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721

FROM builder AS partner-charts
ARG PARTNER_CHART_DEFAULT_BRANCH
RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    # Temporarily clone from our GitHub's main repo, to avoid unnecessary load in git.rancher.io
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    # Charts need to be copied into the sha256 value of git url computed in https://github.com/rancher/rancher/blob/5ebda9ac23c06e9647b586ec38aa51cc9ff9b031/pkg/catalogv2/git/download.go#L102 to create a unique folder per url
    git clone --no-checkout -b $PARTNER_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/partner-charts /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974


FROM builder AS rke2-charts
ARG RKE2_CHART_DEFAULT_BRANCH
RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    # Temporarily clone from our GitHub's main repo, to avoid unnecessary load in git.rancher.io
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    # Charts need to be copied into the sha256 value of git url computed in https://github.com/rancher/rancher/blob/5ebda9ac23c06e9647b586ec38aa51cc9ff9b031/pkg/catalogv2/git/download.go#L102 to create a unique folder per url
    git clone --no-checkout -b $RKE2_CHART_DEFAULT_BRANCH --depth 1 https://git.rancher.io/rke2-charts /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9


FROM builder AS kdm
ARG CATTLE_KDM_BRANCH
RUN mkdir -p /var/lib/rancher-data/driver-metadata && \
    curl -sLf "https://releases.rancher.com/kontainer-driver-metadata/${CATTLE_KDM_BRANCH}/data.json" > /var/lib/rancher-data/driver-metadata/data.json


# Main stage using bci-micro as the base image
FROM final AS base
# Copy binaries and configuration files from builder to micro
COPY --from=chroot-builder-server /chroot/ /

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

ARG ARCH
ARG ETCD_UNSUPPORTED_ARCH
ARG IMAGE_REPO=rancher
ARG IMAGE_NAME=rancher
ARG CHART_DEFAULT_BRANCH
ARG PARTNER_CHART_DEFAULT_BRANCH
ARG RKE2_CHART_DEFAULT_BRANCH
ARG CATTLE_KDM_BRANCH

ENV CATTLE_CHART_DEFAULT_BRANCH=$CHART_DEFAULT_BRANCH
ENV CATTLE_PARTNER_CHART_DEFAULT_BRANCH=$PARTNER_CHART_DEFAULT_BRANCH
ENV CATTLE_RKE2_CHART_DEFAULT_BRANCH=$RKE2_CHART_DEFAULT_BRANCH
ENV CATTLE_MACHINE_VERSION=v0.15.0-rancher145
ENV CATTLE_MACHINE_CHECKSUM_amd64=7cf9e1a42982429d05d66c106d4560269cdfc617f84417013c72a56f1171f668
ENV CATTLE_MACHINE_CHECKSUM_arm64=8f0f1084b5af73c71d982b652b086421ad26468c9e7d27ded00813de133ac174
ENV CATTLE_K3S_VERSION=v1.36.1+k3s1
ENV CATTLE_MACHINE_PROVISION_IMAGE=rancher/machine:${CATTLE_MACHINE_VERSION}
ENV CATTLE_ETCD_VERSION=v3.6.12
ENV CATTLE_ETCD_CHECKSUM_amd64=d2564bb50b58e52fbaf3bde4a9561a1d04e20cdc761f795580e4d615d5d41355
ENV CATTLE_ETCD_CHECKSUM_arm64=147afc710e164fa342ad8561bcca8903f3913bd9fb9ae0c83c857b1d868189cc
# For displaying Helm version in the UI only.
ENV CATTLE_HELM_VERSION=v4.2.0
# make sure the version number is consistent with the one at Line 100 of pkg/data/management/machinedriver_data.go
ENV DOCKER_MACHINE_HARVESTER_VERSION=v1.0.6
ENV DOCKER_MACHINE_HARVESTER_CHECKSUM_amd64=7db00430a1c047e6627681446a99219815156e971043ba7a22a083d69fafe44a
ENV DOCKER_MACHINE_HARVESTER_CHECKSUM_arm64=e536ded30af40bdfa53a1010e39dfe2f72f5add499b564b9516c1385ee3a98c5
ENV CATTLE_KDM_BRANCH=${CATTLE_KDM_BRANCH}
ENV CATTLE_WINS_AGENT_VERSION=v0.16.0-rc.2
ENV CATTLE_WINS_AGENT_CHECKSUM=9078b9ed1dd4bd14b496a9810786a38104ca897e5a40770746b279349bca43b3
ENV CATTLE_WINS_AGENT_CHECKSUM_install=b2837612bc0fb3da630748392923459ec344de98e31a42739418747162de18bd
ENV CATTLE_WINS_AGENT_CHECKSUM_uninstall=a55a1b229d9d80aa64687f9c617a41a4265aff862a4f73f31139d05ddc6cc858
ENV CATTLE_WINS_AGENT_INSTALL_SCRIPT=https://raw.githubusercontent.com/rancher/wins/${CATTLE_WINS_AGENT_VERSION}/install.ps1
ENV CATTLE_WINS_AGENT_UNINSTALL_SCRIPT=https://raw.githubusercontent.com/rancher/wins/${CATTLE_WINS_AGENT_VERSION}/uninstall.ps1
ENV CATTLE_WINS_AGENT_UPGRADE_IMAGE=rancher/wins:${CATTLE_WINS_AGENT_VERSION}
ENV CATTLE_CSI_PROXY_AGENT_VERSION=v1.1.3
ENV CATTLE_CSI_PROXY_AGENT_CHECKSUM=e87a56f9317b73a6cce98a071b8482f27eae07e8bbd3932ab33cb00b13693b73
# make sure the CATTLE_SYSTEM_AGENT_VERSION is consistent with tests/v2/codecoverage/package/Dockerfile
ENV CATTLE_SYSTEM_AGENT_VERSION=v0.15.0
ENV CATTLE_SYSTEM_AGENT_CHECKSUM_amd64=e68913abef52f99b1aee3ccbfafdf49025859d921e89ec99af2244f876e99da0
ENV CATTLE_SYSTEM_AGENT_CHECKSUM_arm64=c9f9b1a0237cd02179edc8f4f0922302994d08932d14e8c327bb1fdc632a2e87
ENV CATTLE_SYSTEM_AGENT_CHECKSUM_install=2deaab5392ca8b24e52438be75fc1414db0bbb674c6e623e6d1fd86f227afc2a
ENV CATTLE_SYSTEM_AGENT_CHECKSUM_uninstall=ded6af4d28ab396dadf3f7c5871aa06c723275f9d7a4ec2d50d0fad9f52988d3
ENV CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX=https://github.com/rancher/system-agent/releases/download
ENV CATTLE_SYSTEM_AGENT_UPGRADE_IMAGE=rancher/system-agent:${CATTLE_SYSTEM_AGENT_VERSION}-suc
ENV CATTLE_SYSTEM_AGENT_INSTALLER_IMAGE=rancher/system-agent-installer-
# make sure the ENV CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT is consistent with pkg/settings/setting.go to utlize the local version of install script downloaded during build/package
ENV CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT=${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/install.sh
ENV CATTLE_SYSTEM_AGENT_UNINSTALL_SCRIPT=${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/system-agent-uninstall.sh
ENV CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION=110.0.0

# AKS,EKS,GKE Operator charts versions
ARG CATTLE_AKS_OPERATOR_VERSION
ENV CATTLE_AKS_OPERATOR_VERSION=$CATTLE_AKS_OPERATOR_VERSION
ARG CATTLE_EKS_OPERATOR_VERSION
ENV CATTLE_EKS_OPERATOR_VERSION=$CATTLE_EKS_OPERATOR_VERSION
ARG CATTLE_GKE_OPERATOR_VERSION
ENV CATTLE_GKE_OPERATOR_VERSION=$CATTLE_GKE_OPERATOR_VERSION
ARG CATTLE_ALI_OPERATOR_VERSION
ENV CATTLE_ALI_OPERATOR_VERSION=$CATTLE_ALI_OPERATOR_VERSION
# System charts minimal version
# Deprecated in favor of CATTLE_FLEET_VERSION.
ENV CATTLE_FLEET_MIN_VERSION=""
ARG CATTLE_FLEET_VERSION
ENV CATTLE_FLEET_VERSION=$CATTLE_FLEET_VERSION
ARG CATTLE_RANCHER_WEBHOOK_VERSION
ENV CATTLE_RANCHER_WEBHOOK_VERSION=$CATTLE_RANCHER_WEBHOOK_VERSION
ARG CATTLE_REMOTEDIALER_PROXY_VERSION
ENV CATTLE_REMOTEDIALER_PROXY_VERSION=$CATTLE_REMOTEDIALER_PROXY_VERSION
ARG CATTLE_RANCHER_TURTLES_VERSION
ENV CATTLE_RANCHER_TURTLES_VERSION=$CATTLE_RANCHER_TURTLES_VERSION
ARG CATTLE_CSP_ADAPTER_MIN_VERSION
ENV CATTLE_CSP_ADAPTER_MIN_VERSION=$CATTLE_CSP_ADAPTER_MIN_VERSION

COPY --from=rancher-charts /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721 /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721
COPY --from=partner-charts /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974
COPY --from=rke2-charts /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9 /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9


RUN case "${ARCH}" in \
        amd64) CATTLE_MACHINE_CHECKSUM="${CATTLE_MACHINE_CHECKSUM_amd64}" ;; \
        arm64) CATTLE_MACHINE_CHECKSUM="${CATTLE_MACHINE_CHECKSUM_arm64}" ;; \
        *) echo "Unsupported: ${ARCH}"; exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/rancher/machine/releases/download/${CATTLE_MACHINE_VERSION}/rancher-machine-${ARCH}.tar.gz"  > /tmp/rancher-machine.tar.gz && \
    printf "%s  %s\n" "${CATTLE_MACHINE_CHECKSUM}" "/tmp/rancher-machine.tar.gz" > machine.sha256 && \
    sha256sum -c machine.sha256 && \
    rm machine.sha256 && \
    tar xvzf /tmp/rancher-machine.tar.gz -C /usr/bin && \
    rm /tmp/rancher-machine.tar.gz && \
    chown root:root /usr/bin/rancher-machine

RUN case "${ARCH}" in \
        amd64) DOCKER_MACHINE_HARVESTER_CHECKSUM="${DOCKER_MACHINE_HARVESTER_CHECKSUM_amd64}" ;; \
        arm64) DOCKER_MACHINE_HARVESTER_CHECKSUM="${DOCKER_MACHINE_HARVESTER_CHECKSUM_arm64}" ;; \
        *) echo "Unsupported: ${ARCH}"; exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/harvester/docker-machine-driver-harvester/releases/download/${DOCKER_MACHINE_HARVESTER_VERSION}/docker-machine-driver-harvester-${ARCH}.tar.gz"  > /tmp/docker-machine-driver-harvester.tar.gz && \
    printf "%s  %s\n" "${DOCKER_MACHINE_HARVESTER_CHECKSUM}" "/tmp/docker-machine-driver-harvester.tar.gz" > machine-harvester.sha256 && \
    sha256sum -c machine-harvester.sha256 && \
    rm machine-harvester.sha256 && \
    tar xvzf /tmp/docker-machine-driver-harvester.tar.gz -C /opt/drivers/management-state/bin && \
    mkdir -p /usr/share/rancher/ui-dashboard/assets/ && \
    ln -s /opt/drivers/management-state/bin/docker-machine-driver-harvester /usr/share/rancher/ui-dashboard/assets/ && \
    rm /tmp/docker-machine-driver-harvester.tar.gz


# Set up K3s: copy the necessary binaries from the K3s image.
COPY --from=rancher/k3s:v1.36.2-k3s1 \
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
    /bin/aux/iptables-apply \
    /bin/aux/iptables-detect.sh \
    /bin/aux/nft \
    /bin/aux/xtables-set-mode.sh \
    /bin/aux/xtables-legacy-multi \
    /bin/aux/xtables-nft-multi \
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
    ln -s /usr/bin/iptables-detect.sh /usr/bin/ip6tables && \
    ln -s /usr/bin/iptables-apply /usr/bin/ip6tables-apply && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables-legacy && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables-legacy-restore && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/ip6tables-legacy-save && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/ip6tables-nft && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/ip6tables-nft-restore && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/ip6tables-nft-save && \
    ln -s /usr/bin/iptables-detect.sh /usr/bin/ip6tables-restore && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/ip6tables-restore-translate && \
    ln -s /usr/bin/iptables-detect.sh /usr/bin/ip6tables-save && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/ip6tables-translate && \
    ln -s /usr/bin/iptables-detect.sh /usr/bin/iptables && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables-legacy && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables-legacy-restore && \
    ln -s /usr/bin/xtables-legacy-multi /usr/bin/iptables-legacy-save && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/iptables-nft && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/iptables-nft-restore && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/iptables-nft-save && \
    ln -s /usr/bin/iptables-detect.sh /usr/bin/iptables-restore && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/iptables-restore-translate && \
    ln -s /usr/bin/iptables-detect.sh /usr/bin/iptables-save && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/iptables-translate && \
    ln -s /usr/bin/xtables-nft-multi /usr/bin/xtables-monitor

RUN mkdir -p /var/lib/rancher/k3s/agent/images/ && \
    case "${ARCH}" in \
        amd64) CATTLE_ETCD_CHECKSUM="${CATTLE_ETCD_CHECKSUM_amd64}" ;; \
        arm64) CATTLE_ETCD_CHECKSUM="${CATTLE_ETCD_CHECKSUM_arm64}" ;; \
        *) echo "Unsupported: ${ARCH}"; exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/etcd-io/etcd/releases/download/${CATTLE_ETCD_VERSION}/etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}.tar.gz" > /tmp/etcd.tar.gz && \
    printf "%s  %s\n" "${CATTLE_ETCD_CHECKSUM}" "/tmp/etcd.tar.gz" > etcd.sha256 && \
    sha256sum -c etcd.sha256 && \
    rm etcd.sha256 && \
    tar xvzf /tmp/etcd.tar.gz --strip-components=1 --no-same-owner -C /usr/bin/ "etcd-${CATTLE_ETCD_VERSION}-linux-${ARCH}/etcdctl" && \
    mkdir -p /var/lib/rancher-data/driver-metadata && \
    rm /tmp/etcd.tar.gz

ENV CATTLE_DASHBOARD_UI_VERSION=master

ENV CATTLE_CLI_VERSION=v2.15.0-rc.1

# Base UI brand used as a fallback env setting (not user facing) to indicate this is a non-prime install
ENV CATTLE_BASE_UI_BRAND=

# RKE2 Prime provisioning default, only set in prime installs
ENV CATTLE_BASE_RKE2_PROVISIONING_PRIME_DEFAULT=

# Cluster Autoscaler related settings, only set in prime installs
ENV CATTLE_CLUSTER_AUTOSCALER_CHART_REPOSITORY=
ENV CATTLE_CLUSTER_AUTOSCALER_IMAGE=

# Please update the api-ui-version in pkg/settings/settings.go when updating the version here.
ENV CATTLE_API_UI_VERSION=1.1.11

RUN mkdir -p /var/log/auditlog
ENV AUDIT_LOG_PATH=/var/log/auditlog/rancher-api-audit.log
ENV AUDIT_LOG_MAXAGE=10
ENV AUDIT_LOG_MAXBACKUP=10
ENV AUDIT_LOG_MAXSIZE=100
ENV AUDIT_LEVEL=0

RUN mkdir -p /usr/share/rancher/ui-dashboard/api-ui && \
    cd /usr/share/rancher/ui-dashboard/api-ui && \
    curl -sL "https://releases.rancher.com/api-ui/${CATTLE_API_UI_VERSION}.tar.gz" | tar xvzf - --strip-components=1 && \
    mkdir -p /usr/share/rancher/ui-dashboard/dashboard && \
    cd /usr/share/rancher/ui-dashboard/dashboard && \
    curl -sL "https://releases.rancher.com/dashboard/${CATTLE_DASHBOARD_UI_VERSION}.tar.gz" | tar xvzf - --strip-components=2 && \
    ln -s dashboard/index.html ../index.html && \
    ln -s dashboard/robots.txt ../robots.txt && \
    ln -s dashboard/humans.txt ../humans.txt && \
    ln -s dashboard/VERSION.txt ../VERSION.txt && \
    cd ../assets && \
    curl -sfL "${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-arm64" -O && \
    printf "%s  %s\n" "${CATTLE_SYSTEM_AGENT_CHECKSUM_arm64}" "rancher-system-agent-arm64" > rancher-system-agent-arm64.sha256 && \
    sha256sum -c rancher-system-agent-arm64.sha256 && \
    rm rancher-system-agent-arm64.sha256 && \
    curl -sfL "${CATTLE_SYSTEM_AGENT_DOWNLOAD_PREFIX}/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-amd64" -O && \
    printf "%s  %s\n" "${CATTLE_SYSTEM_AGENT_CHECKSUM_amd64}" "rancher-system-agent-amd64" > rancher-system-agent-amd64.sha256 && \
    sha256sum -c rancher-system-agent-amd64.sha256 && \
    rm rancher-system-agent-amd64.sha256 && \
    curl -sfL "${CATTLE_SYSTEM_AGENT_INSTALL_SCRIPT}" -o system-agent-install.sh && \
    printf "%s  %s\n" "${CATTLE_SYSTEM_AGENT_CHECKSUM_install}" "system-agent-install.sh" > system-agent-install.sha256 && \
    sha256sum -c system-agent-install.sha256 && \
    rm system-agent-install.sha256 && \
    curl -sfL "${CATTLE_SYSTEM_AGENT_UNINSTALL_SCRIPT}" -o system-agent-uninstall.sh && \
    printf "%s  %s\n" "${CATTLE_SYSTEM_AGENT_CHECKSUM_uninstall}" "system-agent-uninstall.sh" > system-agent-uninstall.sha256 && \
    sha256sum -c system-agent-uninstall.sha256 && \
    rm system-agent-uninstall.sha256 && \
    curl -sfL "https://github.com/rancher/wins/releases/download/${CATTLE_WINS_AGENT_VERSION}/wins.exe" -O && \
    printf "%s  %s\n" "${CATTLE_WINS_AGENT_CHECKSUM}" "wins.exe" > wins.sha256 && \
    sha256sum -c wins.sha256 && \
    rm wins.sha256 && \
    curl -sfL "https://acs-mirror.azureedge.net/csi-proxy/${CATTLE_CSI_PROXY_AGENT_VERSION}/binaries/csi-proxy-${CATTLE_CSI_PROXY_AGENT_VERSION}.tar.gz" -O && \
    printf "%s  %s\n" "${CATTLE_CSI_PROXY_AGENT_CHECKSUM}" "csi-proxy-${CATTLE_CSI_PROXY_AGENT_VERSION}.tar.gz" > csi-proxy.sha256 && \
    sha256sum -c csi-proxy.sha256 && \
    rm csi-proxy.sha256 && \
    curl -sfL "${CATTLE_WINS_AGENT_INSTALL_SCRIPT}" -o wins-agent-install.ps1 && \
    printf "%s  %s\n" "${CATTLE_WINS_AGENT_CHECKSUM_install}" "wins-agent-install.ps1" > wins-install.sha256 && \
    sha256sum -c wins-install.sha256 && \
    rm wins-install.sha256 && \
    curl -sfL "${CATTLE_WINS_AGENT_UNINSTALL_SCRIPT}" -o wins-agent-uninstall.ps1 && \
    printf "%s  %s\n" "${CATTLE_WINS_AGENT_CHECKSUM_uninstall}" "wins-agent-uninstall.ps1" > wins-agent-uninstall.sha256 && \
    sha256sum -c wins-agent-uninstall.sha256 && \
    rm wins-agent-uninstall.sha256

ENV CATTLE_CLI_URL_DARWIN=https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-darwin-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_LINUX=https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-linux-amd64-${CATTLE_CLI_VERSION}.tar.gz
ENV CATTLE_CLI_URL_WINDOWS=https://releases.rancher.com/cli2/${CATTLE_CLI_VERSION}/rancher-windows-386-${CATTLE_CLI_VERSION}.zip

COPY --from=kdm /var/lib/rancher-data/driver-metadata/data.json /var/lib/rancher-data/driver-metadata/data.json

ENV ETCDCTL_API=3

ENV SSL_CERT_DIR=/etc/rancher/ssl
ENV ETCD_UNSUPPORTED_ARCH=${ETCD_UNSUPPORTED_ARCH}


# Enable exporting of the k3s images as part of the build process.
FROM --platform=$BUILDPLATFORM go-builder AS images
WORKDIR /src
ENV CGO_ENABLED=1
ENV CATTLE_K3S_VERSION=v1.36.1+k3s1
RUN zypper -n install libbtrfs-devel libgpgme-devel
COPY hack/airgap/go.mod hack/airgap/go.sum /src/
RUN --mount=type=cache,target=/root/.cache,id=rancher go mod download
COPY hack/airgap/ /src/
RUN --mount=type=cache,target=/root/.cache,id=rancher go build -tags k3s_export -o export-images ./...
RUN ./export-images -k3s-version ${CATTLE_K3S_VERSION} -output /src/k3s-airgap-images.tar

# Output just k3s images
FROM scratch AS k3s-images
COPY --from=images /src/k3s-airgap-images.tar /k3s-airgap-images.tar

# rancher-go-builder is an intermediate stage with go mod ready and Rancher
# dependencies downloaded. Can be used to build both the Rancher server and agent
# binaries.
FROM --platform=$BUILDPLATFORM go-builder AS rancher-go-builder
ARG BUILD_WORKDIR
WORKDIR ${BUILD_WORKDIR}
ARG GODEP_APISERVER_PATH
COPY --from=godep-apiserver / ${GODEP_APISERVER_PATH}
ARG GODEP_LASSO_PATH
COPY --from=godep-lasso / ${GODEP_LASSO_PATH}
ARG GODEP_NORMAN_PATH
COPY --from=godep-norman / ${GODEP_NORMAN_PATH}
ARG GODEP_REMOTEDIALER_PATH
COPY --from=godep-remotedialer / ${GODEP_REMOTEDIALER_PATH}
ARG GODEP_SHEPHERD_PATH
COPY --from=godep-shepherd / ${GODEP_SHEPHERD_PATH}
ARG GODEP_STEVE_PATH
COPY --from=godep-steve / ${GODEP_STEVE_PATH}
ARG GODEP_WRANGLER_PATH
COPY --from=godep-wrangler / ${GODEP_WRANGLER_PATH}
# TEMPORARY (CI cache benchmark): pre-populate /root/.cache (GOMODCACHE +
# GOCACHE, see the ENV in go-builder above) from an externally-supplied
# build context BEFORE go mod download/go build run. This is a plain COPY
# into a normal layer (not a RUN --mount=type=cache), specifically so its
# storage falls under BuildKit's regular (large) layer GC budget rather
# than the much smaller exec.cachemount budget some runner fleets
# configure. Defaults to an empty "scratch" context (see GOCACHE_IN ARG
# above), so this is a no-op unless a real cache dir is explicitly passed
# in via --build-context gocache=<path>.
COPY --from=gocache-in / /root/.cache
# Only invalidate cache if go.mod/go.sum changes.
COPY go.mod go.sum ./
COPY pkg/apis/go.mod pkg/apis/go.sum pkg/apis/
COPY pkg/client/go.mod pkg/client/go.sum pkg/client/
COPY pkg/plan/go.mod pkg/plan/go.sum pkg/plan/
RUN go mod download
RUN cd pkg/apis && go mod download
RUN cd pkg/client && go mod download


FROM --platform=$BUILDPLATFORM rancher-go-builder AS server-build
ARG VERSION
ARG COMMIT
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static -s"
ARG LDFLAGS="-X github.com/rancher/rancher/pkg/version.Version=${VERSION} -X github.com/rancher/rancher/pkg/version.GitCommit=${COMMIT} ${LINKFLAGS}"
ARG GCFLAGS
ARG TARGETOS
ARG TARGETARCH
COPY internal/ internal/
COPY pkg/ pkg/
COPY main.go ./
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" ${GCFLAGS:+-gcflags "$GCFLAGS"} -o /app/rancher

# Output just the server binary
FROM scratch AS server-binary
COPY --from=server-build /app/rancher /bin/rancher

# TEMPORARY (CI cache benchmark): output just the populated go build/mod
# cache directory from server-build, so it can be extracted with a plain
# `docker buildx build --target gocache-out --output type=local,dest=...`
# and handed to actions/cache for cross-run persistence. Remove alongside
# the other TEMPORARY markers in this file once the benchmark is done.
FROM scratch AS gocache-out
COPY --from=server-build /root/.cache /

FROM --platform=$BUILDPLATFORM rancher-go-builder AS agent-build
ARG VERSION
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static -s"
ARG LDFLAGS="-X main.VERSION=${VERSION} $LINKFLAGS"
ARG GCFLAGS
ARG TARGETOS
ARG TARGETARCH
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" ${GCFLAGS:+-gcflags "$GCFLAGS"} -o /app/agent ./cmd/agent

# TEMPORARY (CI cache benchmark, RUN --mount=type=cache variant): parallel
# stages using the ORIGINAL RUN --mount=type=cache mechanism this
# Dockerfile used before any of the COPY-based/bind-mount-based benchmark
# work in this PR (see the "images" stage below for the still-unmodified
# original example) -- paired with a custom buildkitd GC policy set via
# --buildkitd-config-inline on the "Set up Docker Buildx" step in
# pull-request.yml's cache-benchmark-{cold,warm}-cachemount jobs, raising
# the exec.cachemount maxUsedSpace ceiling from this runner fleet's
# default ~488MiB (the root cause diagnosed earlier in this PR for why
# --mount=type=cache alone couldn't be extracted/persisted) to
# comfortably above the ~10GB this cache needs.
#
# If this works, RUN --mount=type=cache is likely the best of the three
# approaches tried in this PR: no COPY/bind-mount ingestion cost into
# /root/.cache at all, and no gocache-out extraction/tar-export tax
# either -- buildkit-cache-dance just does a plain filesystem copy in/out
# of the cache mount's on-disk location, so persisting it across builds
# via actions/cache doesn't require materializing ~10GB as image layers
# at any point.
#
# id=rancher-cache-mount-bench is deliberately NOT id=rancher (the id the
# real "images" stage below still uses) -- keeps this benchmark's cache
# mount fully isolated from the real k3s images build's cache mount, so
# running this benchmark can't affect (or be affected by) the real build
# path.
FROM --platform=$BUILDPLATFORM go-builder AS rancher-go-builder-mount
ARG BUILD_WORKDIR
WORKDIR ${BUILD_WORKDIR}
ARG GODEP_APISERVER_PATH
COPY --from=godep-apiserver / ${GODEP_APISERVER_PATH}
ARG GODEP_LASSO_PATH
COPY --from=godep-lasso / ${GODEP_LASSO_PATH}
ARG GODEP_NORMAN_PATH
COPY --from=godep-norman / ${GODEP_NORMAN_PATH}
ARG GODEP_REMOTEDIALER_PATH
COPY --from=godep-remotedialer / ${GODEP_REMOTEDIALER_PATH}
ARG GODEP_SHEPHERD_PATH
COPY --from=godep-shepherd / ${GODEP_SHEPHERD_PATH}
ARG GODEP_STEVE_PATH
COPY --from=godep-steve / ${GODEP_STEVE_PATH}
ARG GODEP_WRANGLER_PATH
COPY --from=godep-wrangler / ${GODEP_WRANGLER_PATH}
# Only invalidate cache if go.mod/go.sum changes.
COPY go.mod go.sum ./
COPY pkg/apis/go.mod pkg/apis/go.sum pkg/apis/
COPY pkg/client/go.mod pkg/client/go.sum pkg/client/
COPY pkg/plan/go.mod pkg/plan/go.sum pkg/plan/
RUN --mount=type=cache,target=/root/.cache,id=rancher-cache-mount-bench go mod download
RUN --mount=type=cache,target=/root/.cache,id=rancher-cache-mount-bench cd pkg/apis && go mod download
RUN --mount=type=cache,target=/root/.cache,id=rancher-cache-mount-bench cd pkg/client && go mod download


FROM --platform=$BUILDPLATFORM rancher-go-builder-mount AS server-build-mount
ARG VERSION
ARG COMMIT
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static -s"
ARG LDFLAGS="-X github.com/rancher/rancher/pkg/version.Version=${VERSION} -X github.com/rancher/rancher/pkg/version.GitCommit=${COMMIT} ${LINKFLAGS}"
ARG GCFLAGS
ARG TARGETOS
ARG TARGETARCH
COPY internal/ internal/
COPY pkg/ pkg/
COPY main.go ./
RUN --mount=type=cache,target=/root/.cache,id=rancher-cache-mount-bench GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" ${GCFLAGS:+-gcflags "$GCFLAGS"} -o /app/rancher

# Cache-mount variant #2, testing buildkit-cache-dance@v2.1.4 (vendored in
# .github/actions/buildkit-cache-dance-v2/) as a workaround suggested by
# multiple independent reports in the still-open, unresolved upstream issue
# reproducible-containers/buildkit-cache-dance#33 ("Stored cache is empty").
# v2.1.4 predates the current cache-map/per-mount-id architecture and always
# generates a bare `RUN --mount=type=cache,target=...` with NO explicit
# `id=` -- BuildKit defaults the cache-mount id to the target path in that
# case -- so these stages deliberately omit `id=` too, to match v2.1.4's own
# generated Dockerfile shape exactly. Mount target is /root/.cache (NOT a
# separate path) to match GOMODCACHE=/root/.cache/go/modcache and
# GOCACHE=/root/.cache/go/cache set in the shared go-builder base stage --
# an earlier version of this benchmark mistakenly mounted at
# /root/.cache-dance-v2 instead, which `go mod download`/`go build` would
# never actually write into, silently making the whole benchmark measure
# nothing (caught before that run's results were used, per Tom). This job
# runs in its own isolated builder/runner, so reusing the /root/.cache
# target (and therefore the same implicit id string) doesn't collide with
# id=rancher-cache-mount-bench (the explicit-id v3.4.0 benchmark) or
# id=rancher (the real build) -- those live on entirely separate BuildKit
# builder instances.
FROM --platform=$BUILDPLATFORM go-builder AS rancher-go-builder-mount-dance-v2
ARG BUILD_WORKDIR
WORKDIR ${BUILD_WORKDIR}
ARG GODEP_APISERVER_PATH
COPY --from=godep-apiserver / ${GODEP_APISERVER_PATH}
ARG GODEP_LASSO_PATH
COPY --from=godep-lasso / ${GODEP_LASSO_PATH}
ARG GODEP_NORMAN_PATH
COPY --from=godep-norman / ${GODEP_NORMAN_PATH}
ARG GODEP_REMOTEDIALER_PATH
COPY --from=godep-remotedialer / ${GODEP_REMOTEDIALER_PATH}
ARG GODEP_SHEPHERD_PATH
COPY --from=godep-shepherd / ${GODEP_SHEPHERD_PATH}
ARG GODEP_STEVE_PATH
COPY --from=godep-steve / ${GODEP_STEVE_PATH}
ARG GODEP_WRANGLER_PATH
COPY --from=godep-wrangler / ${GODEP_WRANGLER_PATH}
COPY go.mod go.sum ./
COPY pkg/apis/go.mod pkg/apis/go.sum pkg/apis/
COPY pkg/client/go.mod pkg/client/go.sum pkg/client/
COPY pkg/plan/go.mod pkg/plan/go.sum pkg/plan/
RUN --mount=type=cache,target=/root/.cache go mod download
RUN --mount=type=cache,target=/root/.cache cd pkg/apis && go mod download
RUN --mount=type=cache,target=/root/.cache cd pkg/client && go mod download

FROM --platform=$BUILDPLATFORM rancher-go-builder-mount-dance-v2 AS server-build-mount-dance-v2
ARG VERSION
ARG COMMIT
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static -s"
ARG LDFLAGS="-X github.com/rancher/rancher/pkg/version.Version=${VERSION} -X github.com/rancher/rancher/pkg/version.GitCommit=${COMMIT} ${LINKFLAGS}"
ARG GCFLAGS
ARG TARGETOS
ARG TARGETARCH
COPY internal/ internal/
COPY pkg/ pkg/
COPY main.go ./
RUN --mount=type=cache,target=/root/.cache GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" ${GCFLAGS:+-gcflags "$GCFLAGS"} -o /app/rancher

# k3s images step, RUN --mount=type=cache variant -- see
# rancher-go-builder-mount comment above for the full rationale. Mirrors
# the real "images" stage below (same go.mod/go.sum, same go build
# invocation) but uses a distinct cache id (rancher-cache-mount-bench-k3s)
# so this benchmark stays fully isolated from the real k3s images build's
# id=rancher cache mount.
FROM --platform=$BUILDPLATFORM go-builder AS images-mount
WORKDIR /src
ENV CGO_ENABLED=1
ENV CATTLE_K3S_VERSION=v1.36.1+k3s1
RUN zypper -n install libbtrfs-devel libgpgme-devel
COPY hack/airgap/go.mod hack/airgap/go.sum /src/
RUN --mount=type=cache,target=/root/.cache,id=rancher-cache-mount-bench-k3s go mod download
COPY hack/airgap/ /src/
RUN --mount=type=cache,target=/root/.cache,id=rancher-cache-mount-bench-k3s go build -tags k3s_export -o export-images ./...
RUN ./export-images -k3s-version ${CATTLE_K3S_VERSION} -output /src/k3s-airgap-images-mount.tar

# Output just k3s images (cache-mount variant) -- separate target so the
# benchmark can time export-images running against a cache-mount-consumed
# cache without disturbing the real k3s-images target below.
FROM scratch AS k3s-images-mount
COPY --from=images-mount /src/k3s-airgap-images-mount.tar /k3s-airgap-images-mount.tar

# TEMPORARY (CI cache benchmark, bind-mount variant): parallel stages that
# consume the externally-supplied go build/mod cache (gocache-in, see ARG
# GOCACHE_IN above) via RUN --mount=type=bind instead of the plain COPY
# used by rancher-go-builder/server-build/agent-build above. A bind
# mount's writes are NOT committed to the resulting image layer -- they
# are discarded once the RUN instruction completes (see
# https://docs.docker.com/reference/dockerfile/#run---mounttypebind) --
# so these stages can never be used to produce a populated cache for
# extraction the way the COPY-based stages above can, and there is no
# workaround for that within a single RUN/stage. Cache production for
# testing purposes still goes entirely through the existing COPY-based
# server-build + gocache-out mechanism above (unchanged); the resulting
# tar is then fed into THESE bind-mount stages' gocache-in build-context
# to test whether reading an already-populated cache via bind mount (no
# COPY materialization cost, and the cache content itself never becomes
# part of a committed image layer -- smaller image, nothing to garbage
# collect afterward) is faster than the COPY-based approach for a real,
# "warm" build. See cache-benchmark-{cold,warm} jobs in pull-request.yml
# for how cold (empty gocache-in) vs. warm (populated gocache-in, sourced
# from the COPY-based extraction) are compared.
FROM --platform=$BUILDPLATFORM go-builder AS rancher-go-builder-bind
ARG BUILD_WORKDIR
WORKDIR ${BUILD_WORKDIR}
ARG GODEP_APISERVER_PATH
COPY --from=godep-apiserver / ${GODEP_APISERVER_PATH}
ARG GODEP_LASSO_PATH
COPY --from=godep-lasso / ${GODEP_LASSO_PATH}
ARG GODEP_NORMAN_PATH
COPY --from=godep-norman / ${GODEP_NORMAN_PATH}
ARG GODEP_REMOTEDIALER_PATH
COPY --from=godep-remotedialer / ${GODEP_REMOTEDIALER_PATH}
ARG GODEP_SHEPHERD_PATH
COPY --from=godep-shepherd / ${GODEP_SHEPHERD_PATH}
ARG GODEP_STEVE_PATH
COPY --from=godep-steve / ${GODEP_STEVE_PATH}
ARG GODEP_WRANGLER_PATH
COPY --from=godep-wrangler / ${GODEP_WRANGLER_PATH}
# Only invalidate cache if go.mod/go.sum changes.
COPY go.mod go.sum ./
COPY pkg/apis/go.mod pkg/apis/go.sum pkg/apis/
COPY pkg/client/go.mod pkg/client/go.sum pkg/client/
COPY pkg/plan/go.mod pkg/plan/go.sum pkg/plan/
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw go mod download
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw cd pkg/apis && go mod download
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw cd pkg/client && go mod download


FROM --platform=$BUILDPLATFORM rancher-go-builder-bind AS server-build-bind
ARG VERSION
ARG COMMIT
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static -s"
ARG LDFLAGS="-X github.com/rancher/rancher/pkg/version.Version=${VERSION} -X github.com/rancher/rancher/pkg/version.GitCommit=${COMMIT} ${LINKFLAGS}"
ARG GCFLAGS
ARG TARGETOS
ARG TARGETARCH
COPY internal/ internal/
COPY pkg/ pkg/
COPY main.go ./
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" ${GCFLAGS:+-gcflags "$GCFLAGS"} -o /app/rancher


FROM --platform=$BUILDPLATFORM rancher-go-builder-bind AS agent-build-bind
ARG VERSION
ARG TAGS="k8s"
ARG LINKFLAGS="-extldflags -static -s"
ARG LDFLAGS="-X main.VERSION=${VERSION} $LINKFLAGS"
ARG GCFLAGS
ARG TARGETOS
ARG TARGETARCH
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "${TAGS}" -ldflags "${LDFLAGS}" ${GCFLAGS:+-gcflags "$GCFLAGS"} -o /app/agent ./cmd/agent

# k3s images step, bind-mount variant -- see rancher-go-builder-bind
# comment above for the full rationale. Mirrors the "images" stage above
# (same go.mod/go.sum, same go build invocation) but reads its go
# mod/build cache via bind mount from gocache-in instead of
# RUN --mount=type=cache (the original mechanism, GC-limited to ~488MiB
# exec.cachemount storage on this runner fleet -- see the "images" stage
# above, unchanged, still using it) or a COPY.
FROM --platform=$BUILDPLATFORM go-builder AS images-bind
WORKDIR /src
ENV CGO_ENABLED=1
ENV CATTLE_K3S_VERSION=v1.36.1+k3s1
RUN zypper -n install libbtrfs-devel libgpgme-devel
COPY hack/airgap/go.mod hack/airgap/go.sum /src/
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw go mod download
COPY hack/airgap/ /src/
RUN --mount=type=bind,from=gocache-in,target=/root/.cache,rw go build -tags k3s_export -o export-images ./...
RUN ./export-images -k3s-version ${CATTLE_K3S_VERSION} -output /src/k3s-airgap-images-bind.tar

# Output just k3s images (bind-mount variant) -- separate target so the
# benchmark can time export-images running against a bind-mount-consumed
# cache without disturbing the real k3s-images target above.
FROM scratch AS k3s-images-bind
COPY --from=images-bind /src/k3s-airgap-images-bind.tar /k3s-airgap-images-bind.tar


FROM base AS server
ARG VERSION
ARG BASE_REGISTRY
ARG BASE_UI_BRAND
ARG RANCHER_VERSION_TYPE
ARG CATTLE_CLUSTER_AUTOSCALER_CHART_REPOSITORY
ARG CATTLE_CLUSTER_AUTOSCALER_IMAGE
ARG CATTLE_BASE_RKE2_PROVISIONING_PRIME_DEFAULT

ENV CATTLE_BASE_REGISTRY=$BASE_REGISTRY
ENV CATTLE_BASE_UI_BRAND=$BASE_UI_BRAND
ENV RANCHER_VERSION_TYPE=$RANCHER_VERSION_TYPE
ENV CATTLE_SERVER_VERSION=${VERSION}
ENV CATTLE_AGENT_IMAGE=${IMAGE_REPO}/rancher-agent:${VERSION}
ENV CATTLE_SERVER_IMAGE=${IMAGE_REPO}/${IMAGE_NAME}
ENV CATTLE_CLUSTER_AUTOSCALER_CHART_REPOSITORY=$CATTLE_CLUSTER_AUTOSCALER_CHART_REPOSITORY
ENV CATTLE_CLUSTER_AUTOSCALER_IMAGE=$CATTLE_CLUSTER_AUTOSCALER_IMAGE
ENV CATTLE_BASE_RKE2_PROVISIONING_PRIME_DEFAULT=$CATTLE_BASE_RKE2_PROVISIONING_PRIME_DEFAULT

COPY --chown=root:root --chmod=0755 \
    --from=server-build /app/rancher /usr/bin/
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

LABEL "io.artifacthub.package.logo-url"="https://raw.githubusercontent.com/rancher/dashboard/master/shell/static/welcome-cow.svg" \
    "io.artifacthub.package.readme-url"="https://raw.githubusercontent.com/rancher/rancher/${VERSION}/README.md" \
    "org.opencontainers.image.description"="Rancher Manager: complete container management platform." \
    "org.opencontainers.image.title"="Rancher Manager: complete container management platform." \
    "org.opencontainers.image.source"="https://github.com/rancher/rancher" \
    "org.opencontainers.image.version"=${VERSION} \
    "org.opensuse.reference"=rancher/rancher:${VERSION}
ENTRYPOINT ["entrypoint.sh"]


# Main stage using bci-micro as the base image.
FROM final AS agent

# Copy binaries and configuration files from zypper to micro.
COPY --from=chroot-builder-agent /chroot/ /

# Test that some of the dependency binaries were copied
# and are working on the target image.
RUN /usr/bin/unshare --version && \
    /usr/bin/mount --version && \
    /usr/bin/umount --version && \
    /usr/bin/nsenter --version

ARG ARCH
ENV KUBECTL_VERSION=v1.36.0
ENV KUBECTL_CHECKSUM_amd64=123d8c8844f46b1244c547fffb3c17180c0c26dac9890589fe7e67763298748e
ENV KUBECTL_CHECKSUM_arm64=9f9d9c44a7b5264515ac9da5991584e2395bd50662e651132337e7b4d0c56f8f

RUN case "${ARCH}" in \
        amd64) KUBECTL_CHECKSUM="${KUBECTL_CHECKSUM_amd64}" ;; \
        arm64) KUBECTL_CHECKSUM="${KUBECTL_CHECKSUM_arm64}" ;; \
        *) echo "Unsupported: ${ARCH}"; exit 1 ;; \
    esac && \
    curl -fsSL "https://dl.k8s.io/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl" > /usr/bin/kubectl && \
    printf "%s  %s\n" "${KUBECTL_CHECKSUM}" "/usr/bin/kubectl" > kubectl.sha256 && \
    sha256sum -c kubectl.sha256 && \
    rm kubectl.sha256 && \
    chmod +x /usr/bin/kubectl

LABEL io.cattle.agent=true

ARG RANCHER_REPO=rancher
ARG VERSION
ENV AGENT_IMAGE=${RANCHER_REPO}/rancher-agent:${VERSION}
# For now, this value needs to be manually synced with the one in the main Dockerfile. This pins downstream webhook's version.
ARG CATTLE_RANCHER_WEBHOOK_VERSION
ENV CATTLE_RANCHER_WEBHOOK_VERSION=$CATTLE_RANCHER_WEBHOOK_VERSION
ARG CATTLE_RANCHER_TURTLES_VERSION
ENV CATTLE_RANCHER_TURTLES_VERSION=$CATTLE_RANCHER_TURTLES_VERSION
ENV SSL_CERT_DIR=/etc/kubernetes/ssl/certs
COPY --from=rancher-charts /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721 /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721
COPY --from=partner-charts /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974
COPY --from=rke2-charts /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9 /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9
COPY --from=kdm /var/lib/rancher-data/driver-metadata/data.json /var/lib/rancher-data/driver-metadata/data.json
COPY --from=agent-build /app/agent /usr/bin/
COPY package/loglevel package/run.sh /usr/bin/
WORKDIR /var/lib/rancher

LABEL "io.artifacthub.package.logo-url"="https://raw.githubusercontent.com/rancher/dashboard/master/shell/static/welcome-cow.svg" \
    "io.artifacthub.package.readme-url"="https://raw.githubusercontent.com/rancher/rancher/${VERSION}/README.md" \
    "org.opencontainers.image.description"="Rancher Manager Agent: complete container management platform." \
    "org.opencontainers.image.title"="Rancher Manager Agent: complete container management platform." \
    "org.opencontainers.image.source"="https://github.com/rancher/rancher" \
    "org.opencontainers.image.version"=${VERSION} \
    "org.opensuse.reference"=rancher/rancher-agent:${VERSION}

ENTRYPOINT ["run.sh"]
