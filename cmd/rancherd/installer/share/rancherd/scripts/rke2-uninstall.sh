#!/bin/sh

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    echo "$(basename "${0}"): must be run as root" >&2
    exit 1
fi

if [ -e "/etc/systemd/system/${INSTALL_RKE2_NAME}.env" ]; then
    . "/etc/systemd/system/${INSTALL_RKE2_NAME}.env"
fi

: "${INSTALL_RKE2_ROOT:="/usr/local"}"
: "${INSTALL_RKE2_NAME:="rancherd"}"

if [ -e "${rke2_killall:="$(dirname "$0")/rke2-killall.sh"}" ]; then
  eval "${rke2_killall}"
fi

if command -v systemctl >/dev/null 2>&1; then
    systemctl disable "${INSTALL_RKE2_NAME}" || true
    systemctl reset-failed "${INSTALL_RKE2_NAME}" || true
    systemctl daemon-reload
fi

# remove service files
rm -f "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service"
rm -rf "${INSTALL_RKE2_ROOT}/lib/systemd/system/${INSTALL_RKE2_NAME}.service.d"

if (ls ${INSTALL_RKE2_ROOT}/lib/systemd/system/rancherd*.service || ls /etc/init.d/rancherd*) >/dev/null 2>&1; then
    set +x; echo 'Additional rancherd services installed, skipping uninstall of rancherd'; set -x
    exit
fi

set -e

rm -rf /etc/rancher/rke2
rm -rf /var/lib/kubelet
rm -rf /var/lib/rancher/rke2
rm -f "/etc/sysctl.d/*-${INSTALL_RKE2_NAME}-cis.conf"
rm -f "${INSTALL_RKE2_ROOT}/bin/rancherd"
rm -f "/etc/systemd/system/${INSTALL_RKE2_NAME}.env"
