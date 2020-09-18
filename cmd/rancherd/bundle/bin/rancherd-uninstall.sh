#!/bin/sh
set -ex

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    echo "$(basename "${0}"): must be run as root" >&2
    exit 1
fi

: "${INSTALL_RANCHERD_ROOT:="/usr/local"}"

uninstall_killall()
{
    _killall="$(dirname "$0")/rancherd-killall.sh"
    if [ -e "${_killall}" ]; then
      eval "${_killall}"
    fi
}

uninstall_disable_services()
{
    if command -v systemctl >/dev/null 2>&1; then
        systemctl disable rancherd-server || true
        systemctl disable rancherd-agent || true
        systemctl reset-failed rancherd-server || true
        systemctl reset-failed rancherd-agent || true
        systemctl daemon-reload
    fi
}

uninstall_remove_files()
{
    rm -f "${INSTALL_RANCHERD_ROOT}/lib/systemd/system/rancherd*.service"
    rm -f "${INSTALL_RANCHERD_ROOT}/bin/rancherd"
    rm -f "${INSTALL_RANCHERD_ROOT}/bin/rancherd-killall.sh"
    rm -rf "${INSTALL_RANCHERD_ROOT}/share/rancherd"
    rm -rf /etc/rancher/rke2
    rm -rf /var/lib/kubelet
    rm -rf /var/lib/rancher/rke2
}

uninstall_remove_self()
{
    rm -f "${INSTALL_RANCHERD_ROOT}/bin/rancherd-uninstall.sh"
}

uninstall_killall
trap uninstall_remove_self EXIT
uninstall_disable_services
uninstall_remove_files
