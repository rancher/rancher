#!/bin/sh

if [ "${DEBUG}" = 1 ]; then
  set -x
fi

# Environment variables:
#   - RKE2_*
#     Environment variables which begin with RKE2_ will be preserved for the
#     systemd service to use. Setting RKE2_URL without explicitly setting
#     a systemd exec command will default the command to "agent", and we
#     enforce that RKE2_TOKEN or RKE2_CLUSTER_SECRET is also set.
#
#   - INSTALL_RKE2_SKIP_ENABLE
#     If set to true will not enable or start rke2 service.
#     Default is "false".
#
#   - INSTALL_RKE2_SKIP_START
#     If set to true will not start rke2 service.
#     Default is "false".
#
#   - INSTALL_RKE2_VERSION
#     Version of rke2 to download from github. Will attempt to download from the
#     stable channel if not specified.
#
#   - INSTALL_RKE2_ROOT
#     Filesystem location to unpack tarball.
#     Default is "/usr/local".
#
#   - INSTALL_RKE2_NAME
#     Name of systemd service to create.
#     Default is "rancherd".
#
#   - INSTALL_RKE2_TYPE
#     Type of rke2 service. Can be either "server" or "agent".
#     Default is "server" when unspecified and $RKE2_URL is empty.
#     Default is "agent" when unspecified and $RKE2_URL not empty.
#

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    echo "$(basename "${0}"): must be run as root" >&2
    exit 1
fi

# if no systemd then bail
command -v systemctl >/dev/null 2>&1 || return

set -e

: "${INSTALL_RKE2_NAME:="rancherd"}"
: "${INSTALL_RKE2_ROOT:="/usr/local"}"

INSTALL_RKE2_ROOT="$(realpath "${INSTALL_RKE2_ROOT}")"

if [ -z "${INSTALL_RKE2_TYPE}" ]; then
    if [ -z "${RKE2_URL}" ]; then
        INSTALL_RKE2_TYPE="server"
    else
        INSTALL_RKE2_TYPE="agent"
    fi
fi

# should we assume selinux?
if [ -z "${RKE2_SELINUX}" ] && command -v getenforce >/dev/null 2>&1; then
    if [ -f /usr/share/selinux/packages/rke2.pp ] && [ "$(getenforce)" != "Disabled" ]; then
        RKE2_SELINUX=true
    fi
fi

mkdir -p "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service.d"

# setup service/installation environment file
if [ -d "${INSTALL_RKE2_ROOT}/lib/systemd/system" ]; then
cat <<-EOF > "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.env"
HOME=/root
INSTALL_RKE2_ROOT=${INSTALL_RKE2_ROOT}
INSTALL_RKE2_NAME=${INSTALL_RKE2_NAME}
EOF
env | grep -E '^RKE2_' | sort >> "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.env"
fi

# setup the service file
cp -f "${INSTALL_RKE2_ROOT}/share/rancherd/${INSTALL_RKE2_NAME}.service" "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service"
if [ "${RKE2_SELINUX}" = "true" ]; then
    chcon -t container_unit_file_t "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service" || true
fi

# setup the service overrides
cat <<-EOF > "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service.d/00-install.conf"
[Service]
EnvironmentFile=-${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.env
ExecStart=
ExecStart=${INSTALL_RKE2_ROOT}/bin/rancherd ${INSTALL_RKE2_TYPE}
EOF

# enable the cis profile
if [ -n "${RKE2_CIS_PROFILE}" ]; then
    for conf in "${INSTALL_RKE2_ROOT}"/etc/sysctl.d/*.conf; do
        cp -f "${conf}" "/etc/sysctl.d/${INSTALL_RKE2_CIS_SYSCTL_PREFIX:="30"}-$(basename "${conf}")"
    done
    systemctl restart systemd-sysctl >/dev/null
fi

# enable the service
if [ "${INSTALL_RKE2_SKIP_ENABLE="false"}" = "true" ]; then
    return
fi
systemctl enable "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service" > /dev/null
systemctl daemon-reload >/dev/null

# start the service
if [ "${INSTALL_RKE2_SKIP_START="false"}" != "true" ]; then
    systemctl restart "${INSTALL_RKE2_NAME}"
fi
