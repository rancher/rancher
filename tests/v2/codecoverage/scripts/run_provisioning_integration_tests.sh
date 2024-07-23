#!/bin/bash
set -e -x

mkdir -p /etc/rancher/k3s

corral vars ranchertestcoverage kubeconfig | base64 --decode > /etc/rancher/k3s/k3s.yaml

TB_ORG=rancher

if [ -z "${V2PROV_TEST_DIST}" ] || [ "${V2PROV_TEST_DIST}" = "k3s" ]; then
  V2PROV_TEST_DIST=k3s
  AIRGAP=-airgap
  TB_ORG=k3s-io
else
  LINUX=.linux
fi

if [ -z "${V2PROV_TEST_RUN_REGEX}" ]; then
  V2PROV_TEST_RUN_REGEX="^Test_(General|Provisioning)_.*$"
fi

RUNARG="-run ${V2PROV_TEST_RUN_REGEX}"

export DIST=${V2PROV_TEST_DIST}
export SOME_K8S_VERSION=${SOME_K8S_VERSION}
export TB_ORG=${TB_ORG}

eval "$(grep '^ENV CATTLE_SYSTEM_AGENT' package/Dockerfile | awk '{print "export " $2 "=" $3}')"
eval "$(grep '^ENV CATTLE_WINS_AGENT' package/Dockerfile | awk '{print "export " $2 "=" $3}')"
eval "$(grep '^ENV CATTLE_CSI_PROXY_AGENT' package/Dockerfile | awk '{print "export " $2 "=" $3}')"
eval "$(grep '^ENV CATTLE_KDM_BRANCH' package/Dockerfile | awk '{print "export " $2 "=" $3}')"

if [ -z "${SOME_K8S_VERSION}" ]; then
# Get the last release for $DIST, which is usually the latest version or an experimental version.
# Previously this would use channels, but channels no longer reflect the latest version since
# https://github.com/rancher/rancher/issues/36827 has added appDefaults. We do not use appDefaults
# here for simplicity's sake, as it requires semver parsing & matching. The last release should
# be good enough for our needs.
export SOME_K8S_VERSION=$(curl -sS https://raw.githubusercontent.com/rancher/kontainer-driver-metadata/release-v2.9/data/data.json | jq -r ".$DIST.releases[-1].version")
fi

echo "Starting rancher server for provisioning-tests using $SOME_K8S_VERSION"
touch /tmp/rancher.log

mkdir -p /var/lib/rancher/$DIST/agent/images
grep PodTestImage ./tests/v2prov/defaults/defaults.go | cut -f2 -d'"' > /var/lib/rancher/$DIST/agent/images/pull.txt
grep MachineProvisionImage ./pkg/settings/setting.go | cut -f4 -d'"' >> /var/lib/rancher/$DIST/agent/images/pull.txt
mkdir -p /usr/share/rancher/ui/assets
curl -sLf https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-amd64 -o /usr/share/rancher/ui/assets/rancher-system-agent-amd64
curl -sLf https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/system-agent-uninstall.sh -o /usr/share/rancher/ui/assets/system-agent-uninstall.sh

echo Running provisioning-tests $RUNARG

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
go test $RUNARG -v -failfast -timeout 60m ./tests/v2prov/tests/... || {
    echo -e "-----RANCHER-LOG-DUMP-START-----"
    cat /tmp/rancher.log | gzip | base64 -w 0
    echo -e "\n-----RANCHER-LOG-DUMP-END-----"
    exit 1
}
