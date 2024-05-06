package image

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// OriginMap is a mapping between images used by Rancher
// and their origin repository. This map must be updated
// when net-new images are added to Rancher or images are
// no longer used by Rancher. The keys of this map should
// mirror the images listed in rancher-images.txt.
// Some net-new images may be resolved via the check-origins script
// (dapper check-origins).

// images should be double-checked and confirmed
// by the team which owns the image.

var OriginMap = map[string]string{
	"aci-containers-controller":                               "https://github.com/noironetworks/aci-containers",
	"aci-containers-host":                                     "https://github.com/noironetworks/aci-containers",
	"aks-operator":                                            "https://github.com/rancher/aks-operator",
	"backup-restore-operator":                                 "https://github.com/rancher/backup-restore-operator",
	"calico-cni":                                              "https://github.com/rancher/calico-cni",
	"cis-operator":                                            "https://github.com/rancher/cis-operator",
	"cnideploy":                                               "https://github.com/containernetworking/plugins",
	"coreos-kube-state-metrics":                               "https://github.com/kubernetes/kube-state-metrics",
	"coreos-prometheus-config-reloader":                       "https://github.com/prometheus-operator/prometheus-operator/pkgs/container/prometheus-config-reloader",
	"coreos-prometheus-operator":                              "https://github.com/prometheus-operator/prometheus-operator",
	"eks-operator":                                            "https://github.com/rancher/eks-operator",
	"externalip-webhook":                                      "https://github.com/rancher/externalip-webhook",
	"flannel-cni":                                             "https://github.com/rancher/flannel-cni",
	"fleet":                                                   "https://github.com/rancher/fleet",
	"fleet-agent":                                             "https://github.com/rancher/fleet",
	"fluent-bit":                                              "https://github.com/fluent/fluent-bit",
	"gbp-server":                                              "https://github.com/noironetworks/aci-containers/tree/master/pkg/gbpserver",
	"gitjob":                                                  "https://github.com/rancher/gitjob",
	"gke-operator":                                            "https://github.com/rancher/gke-operator",
	"grafana-grafana":                                         "https://github.com/grafana/grafana",
	"hardened-addon-resizer":                                  "https://github.com/rancher/image-build-addon-resizer",
	"hardened-calico":                                         "https://github.com/rancher/image-build-calico",
	"hardened-cluster-autoscaler":                             "https://github.com/kubernetes/autoscaler",
	"hardened-cni-plugins":                                    "https://github.com/rancher/image-build-cni-plugins",
	"hardened-coredns":                                        "https://github.com/rancher/image-build-coredns",
	"hardened-dns-node-cache":                                 "https://github.com/rancher/image-build-dns-nodecache",
	"hardened-etcd":                                           "https://github.com/rancher/image-build-etcd",
	"hardened-flannel":                                        "https://github.com/rancher/image-build-flannel",
	"hardened-ib-sriov-cni":                                   "https://github.com/rancher/image-build-ib-sriov-cni",
	"hardened-k8s-metrics-server":                             "https://github.com/rancher/image-build-k8s-metrics-server",
	"hardened-kubernetes":                                     "https://github.com/rancher/image-build-kubernetes",
	"hardened-multus-cni":                                     "https://github.com/rancher/image-build-multus",
	"hardened-node-feature-discovery":                         "https://github.com/rancher/image-build-node-feature-discovery",
	"hardened-sriov-cni":                                      "https://github.com/rancher/image-build-sriov-cni",
	"hardened-sriov-network-config-daemon":                    "https://github.com/openshift/sriov-network-operator/blob/master/Dockerfile.sriov-network-config-daemon",
	"hardened-sriov-network-device-plugin":                    "https://github.com/rancher/image-build-sriov-network-device-plugin",
	"hardened-sriov-network-operator":                         "https://github.com/rancher/image-build-sriov-operator",
	"hardened-sriov-network-resources-injector":               "https://github.com/rancher/image-build-sriov-network-resources-injector",
	"hardened-sriov-network-webhook":                          "https://github.com/openshift/sriov-network-webhook",
	"hardened-whereabouts":                                    "https://github.com/rancher/image-build-whereabouts",
	"harvester-cloud-provider":                                "https://github.com/harvester/cloud-provider-harvester",
	"harvester-csi-driver":                                    "https://github.com/harvester/harvester-csi-driver",
	"helm-project-operator":                                   "https://github.com/rancher/helm-project-operator",
	"hyperkube":                                               "https://github.com/rancher/hyperkube",
	"istio-1.5-migration":                                     "https://github.com/rancher/istio-1.5-migration",
	"istio-installer":                                         "https://github.com/rancher/istio-installer",
	"istio-kubectl":                                           "https://github.com/istio/istio",
	"jimmidyson-configmap-reload":                             "https://github.com/jimmidyson/configmap-reload",
	"k3s-upgrade":                                             "https://github.com/rancher/k3s-upgrade",
	"klipper-helm":                                            "https://github.com/rancher/klipper-helm",
	"klipper-lb":                                              "https://github.com/rancher/klipper-lb",
	"kube-api-auth":                                           "https://github.com/rancher/kube-api-auth",
	"kubectl":                                                 "https://github.com/rancher/kubectl",
	"kubelet-pause":                                           "https://github.com/kubernetes/kubernetes",
	"library-nginx":                                           "https://github.com/nginx/nginx",
	"local-path-provisioner":                                  "https://github.com/rancher/local-path-provisioner",
	"longhornio-csi-attacher":                                 "https://github.com/longhorn/longhorn",
	"longhornio-csi-node-driver-registrar":                    "https://github.com/longhorn/node-driver-registrar",
	"longhornio-csi-provisioner":                              "https://github.com/longhorn/longhorn",
	"longhornio-csi-resizer":                                  "https://github.com/longhorn/longhorn",
	"machine":                                                 "https://github.com/rancher/machine",
	"mirrored-amazon-aws-cli":                                 "https://github.com/aws/aws-cli",
	"mirrored-appscode-kubed":                                 "https://github.com/kubeops/config-syncer",
	"mirrored-banzaicloud-fluentd":                            "https://github.com/fluent/fluentd",
	"mirrored-banzaicloud-logging-operator":                   "https://github.com/banzaicloud/logging-operator",
	"mirrored-bci-busybox":                                    "https://build.opensuse.org/package/show/devel:BCI:SLE-15-SP4/busybox-image",
	"mirrored-bci-micro":                                      "https://build.opensuse.org/package/show/devel:BCI:SLE-15-SP4/micro-image",
	"mirrored-calico-apiserver":                               "https://github.com/projectcalico/calico/tree/master/apiserver",
	"mirrored-calico-cni":                                     "https://github.com/projectcalico/calico",
	"mirrored-calico-csi":                                     "https://github.com/projectcalico/calico",
	"mirrored-calico-ctl":                                     "https://github.com/projectcalico/calico/tree/master/calicoctl",
	"mirrored-calico-kube-controllers":                        "https://github.com/projectcalico/calico/tree/master/kube-controllers",
	"mirrored-calico-node":                                    "https://github.com/projectcalico/calico/tree/master/node",
	"mirrored-calico-node-driver-registrar":                   "https://github.com/projectcalico/calico",
	"mirrored-calico-operator":                                "https://github.com/projectcalico/calico",
	"mirrored-calico-pod2daemon-flexvol":                      "https://github.com/projectcalico/calico/tree/master/pod2daemon",
	"mirrored-calico-typha":                                   "https://github.com/projectcalico/calico/tree/master/typha",
	"mirrored-cilium-certgen":                                 "https://github.com/cilium/certgen",
	"mirrored-cilium-cilium":                                  "https://github.com/cilium/cilium",
	"mirrored-cilium-cilium-envoy":                            "https://github.com/cilium/proxy",
	"mirrored-cilium-cilium-etcd-operator":                    "https://github.com/cilium/cilium-etcd-operator",
	"mirrored-cilium-clustermesh-apiserver":                   "https://github.com/cilium/clustermesh-apiserver",
	"mirrored-cilium-hubble-relay":                            "https://github.com/cilium/hubble",
	"mirrored-cilium-hubble-ui":                               "https://github.com/cilium/hubble-ui",
	"mirrored-cilium-hubble-ui-backend":                       "https://github.com/cilium/hubble-ui",
	"mirrored-cilium-kvstoremesh":                             "https://github.com/cilium/cilium",
	"mirrored-cilium-operator-aws":                            "https://github.com/cilium/cilium/tree/master/pkg/aws",
	"mirrored-cilium-operator-azure":                          "https://github.com/cilium/cilium/tree/master/pkg/azure",
	"mirrored-cilium-operator-generic":                        "https://github.com/cilium/cilium",
	"mirrored-cilium-startup-script":                          "https://github.com/cilium/cilium",
	"mirrored-cloud-provider-vsphere-cpi-release-manager":     "https://github.com/kubernetes/cloud-provider-vsphere/blob/master/cluster/images/controller-manager/Dockerfile",
	"mirrored-cloud-provider-vsphere-csi-release-driver":      "https://github.com/kubernetes-sigs/vsphere-csi-driver",
	"mirrored-cloud-provider-vsphere-csi-release-syncer":      "https://github.com/kubernetes-sigs/vsphere-csi-driver/tree/master/pkg/syncer",
	"mirrored-cluster-api-controller":                         "https://github.com/kubernetes-sigs/cluster-api",
	"mirrored-cluster-proportional-autoscaler":                "https://github.com/kubernetes-sigs/cluster-proportional-autoscaler",
	"mirrored-coredns-coredns":                                "https://github.com/coredns/coredns",
	"mirrored-coreos-etcd":                                    "https://github.com/etcd-io/etcd",
	"mirrored-coreos-flannel":                                 "https://github.com/flannel-io/flannel",
	"mirrored-coreos-prometheus-config-reloader":              "https://github.com/prometheus-operator/prometheus-operator/pkgs/container/prometheus-config-reloader",
	"mirrored-coreos-prometheus-operator":                     "https://github.com/prometheus-operator/prometheus-operator",
	"mirrored-curlimages-curl":                                "https://github.com/curl/curl-docker",
	"mirrored-dexidp-dex":                                     "https://github.com/dexidp/dex",
	"mirrored-directxman12-k8s-prometheus-adapter":            "https://hub.docker.com/r/directxman12/k8s-prometheus-adapter/tags",
	"mirrored-elemental-operator":                             "https://github.com/rancher/elemental-operator",
	"mirrored-elemental-seedimage-builder":                    "https://github.com/rancher/elemental-operator",
	"mirrored-epinio-epinio-server":                           "https://github.com/epinio/epinio",
	"mirrored-epinio-epinio-ui":                               "https://github.com/kubeops/config-syncer",
	"mirrored-epinio-epinio-unpacker":                         "https://github.com/epinio/epinio",
	"mirrored-flannelcni-flannel":                             "https://github.com/flannel-io/flannel",
	"mirrored-flannel-flannel":                                "https://github.com/flannel-io/flannel",
	"mirrored-fluent-fluent-bit":                              "https://github.com/fluent/fluent-bit",
	"mirrored-grafana-grafana":                                "https://github.com/grafana/grafana",
	"mirrored-grafana-grafana-image-renderer":                 "https://github.com/grafana/grafana-image-renderer",
	"mirrored-idealista-prom2teams":                           "https://github.com/idealista/prom2teams",
	"mirrored-ingress-nginx-kube-webhook-certgen":             "https://github.com/jet/kube-webhook-certgen",
	"mirrored-istio-citadel":                                  "https://github.com/istio/istio/tree/master",
	"mirrored-istio-coredns-plugin":                           "https://github.com/istio/istio",
	"mirrored-istio-galley":                                   "https://github.com/istio/istio",
	"mirrored-istio-install-cni":                              "https://github.com/istio/istio",
	"mirrored-istio-kubectl":                                  "https://github.com/istio/istio",
	"mirrored-istio-mixer":                                    "https://github.com/envoyproxy/envoy",
	"mirrored-istio-node-agent-k8s":                           "https://github.com/istio/istio/tree/master/pkg/istio-agent",
	"mirrored-istio-pilot":                                    "https://github.com/istio/istio/tree/master/pilot",
	"mirrored-istio-proxyv2":                                  "https://github.com/istio/istio/blob/master/pilot/docker/Dockerfile.proxyv2",
	"mirrored-istio-sidecar_injector":                         "https://github.com/istio/istio/blob/1.5.9/pilot/pkg/bootstrap/sidecarinjector.go",
	"mirrored-jaegertracing-all-in-one":                       "https://github.com/jaegertracing/jaeger/tree/main/cmd/all-in-one",
	"mirrored-jetstack-cert-manager-controller":               "https://github.com/cert-manager/cert-manager",
	"mirrored-jimmidyson-configmap-reload":                    "https://github.com/jimmidyson/configmap-reload",
	"mirrored-k8s-dns-dnsmasq-nanny":                          "https://github.com/kubernetes/dns",
	"mirrored-k8s-dns-kube-dns":                               "https://github.com/kubernetes/dns",
	"mirrored-k8s-dns-node-cache":                             "https://github.com/kubernetes/dns",
	"mirrored-k8s-dns-sidecar":                                "https://github.com/kubernetes/dns",
	"mirrored-k8scsi-csi-node-driver-registrar":               "https://github.com/kubernetes-csi/node-driver-registrar",
	"mirrored-k8scsi-csi-resizer":                             "https://github.com/kubernetes-csi/external-resizer",
	"mirrored-k8scsi-livenessprobe":                           "https://github.com/kubernetes-csi/livenessprobe",
	"mirrored-kiali-kiali":                                    "https://github.com/kiali/kiali",
	"mirrored-kiwigrid-k8s-sidecar":                           "https://github.com/kiwigrid/k8s-sidecar",
	"mirrored-kube-logging-logging-operator":                  "https://github.com/kube-logging/logging-operator",
	"mirrored-kube-rbac-proxy":                                "https://github.com/brancz/kube-rbac-proxy",
	"mirrored-kube-state-metrics-kube-state-metrics":          "https://github.com/kubernetes/kube-state-metrics",
	"mirrored-kube-vip-kube-vip-iptables":                     "https://github.com/kube-vip/kube-vip",
	"mirrored-kubernetes-external-dns":                        "https://github.com/kubernetes-sigs/external-dns",
	"mirrored-library-busybox":                                "https://github.com/docker-library/busybox",
	"mirrored-library-nginx":                                  "https://github.com/docker-library/nginx",
	"mirrored-library-registry":                               "https://github.com/docker-library/official-images",
	"mirrored-library-traefik":                                "https://github.com/traefik/traefik-library-image",
	"mirrored-longhornio-backing-image-manager":               "https://github.com/longhorn/backing-image-manager",
	"mirrored-longhornio-csi-attacher":                        "https://github.com/longhorn/external-attacher",
	"mirrored-longhornio-csi-node-driver-registrar":           "https://github.com/longhorn/node-driver-registrar",
	"mirrored-longhornio-csi-provisioner":                     "https://github.com/longhorn/external-provisioner",
	"mirrored-longhornio-csi-resizer":                         "https://github.com/longhorn/external-resizer",
	"mirrored-longhornio-csi-snapshotter":                     "https://github.com/longhorn/external-snapshotter",
	"mirrored-longhornio-livenessprobe":                       "https://github.com/longhorn/longhorn",
	"mirrored-longhornio-longhorn-engine":                     "https://github.com/longhorn/longhorn-engine",
	"mirrored-longhornio-longhorn-instance-manager":           "https://github.com/longhorn/longhorn-instance-manager",
	"mirrored-longhornio-longhorn-manager":                    "https://github.com/longhorn/longhorn-manager",
	"mirrored-longhornio-longhorn-share-manager":              "https://github.com/longhorn/longhorn-share-manager",
	"mirrored-longhornio-longhorn-ui":                         "https://github.com/longhorn/longhorn-ui",
	"mirrored-longhornio-support-bundle-kit":                  "https://github.com/longhorn/longhorn",
	"mirrored-longhornio-openshift-origin-oauth-proxy":        "https://github.com/longhorn/longhorn",
	"mirrored-messagebird-sachet":                             "https://github.com/messagebird/sachet",
	"mirrored-metrics-server":                                 "https://github.com/kubernetes-sigs/metrics-server",
	"mirrored-minio-mc":                                       "https://github.com/minio/mc",
	"mirrored-minio-minio":                                    "https://github.com/minio/minio",
	"mirrored-neuvector-controller":                           "https://github.com/neuvector/neuvector",
	"mirrored-neuvector-enforcer":                             "https://github.com/neuvector/neuvector",
	"mirrored-neuvector-manager":                              "https://github.com/neuvector/manager",
	"mirrored-neuvector-scanner":                              "https://github.com/neuvector/scanner",
	"mirrored-neuvector-updater":                              "https://github.com/neuvector/neuvector",
	"mirrored-neuvector-prometheus-exporter":                  "https://github.com/neuvector/prometheus-exporter",
	"mirrored-neuvector-registry-adapter":                     "https://github.com/neuvector/registry-adapter",
	"mirrored-nginx-ingress-controller-defaultbackend":        "https://github.com/rancher/ingress-nginx",
	"mirrored-openpolicyagent-gatekeeper":                     "https://github.com/open-policy-agent/gatekeeper",
	"mirrored-openpolicyagent-gatekeeper-crds":                "https://github.com/open-policy-agent/gatekeeper",
	"mirrored-openzipkin-zipkin":                              "https://github.com/openzipkin/zipkin",
	"mirrored-paketobuildpacks-builder":                       "https://github.com/paketo-buildpacks/full-builder",
	"mirrored-pause":                                          "https://hub.docker.com/r/kubernetes/pause",
	"mirrored-prom-node-exporter":                             "https://github.com/prometheus/node_exporter",
	"mirrored-prom-prometheus":                                "https://github.com/prometheus/prometheus",
	"mirrored-prometheus-adapter-prometheus-adapter":          "https://github.com/kubernetes-sigs/prometheus-adapter",
	"mirrored-prometheus-alertmanager":                        "https://github.com/prometheus/alertmanager",
	"mirrored-prometheus-node-exporter":                       "https://github.com/prometheus/node_exporter",
	"mirrored-prometheus-operator-prometheus-config-reloader": "https://github.com/prometheus-operator/prometheus-operator/pkgs/container/prometheus-config-reloader",
	"mirrored-prometheus-operator-prometheus-operator":        "https://github.com/prometheus-operator/prometheus-operator",
	"mirrored-prometheus-prometheus":                          "https://github.com/prometheus/prometheus",
	"mirrored-pstauffer-curl":                                 "https://github.com/pstauffer/docker-curl",
	"mirrored-s3gw-s3gw":                                      "https://github.com/aquarist-labs/s3gw",
	"mirrored-sig-storage-csi-attacher":                       "https://github.com/kubernetes-csi/external-attacher",
	"mirrored-sig-storage-csi-node-driver-registrar":          "https://github.com/kubernetes-csi/node-driver-registrar",
	"mirrored-sig-storage-csi-provisioner":                    "https://github.com/kubernetes-csi/external-provisioner",
	"mirrored-sig-storage-csi-resizer":                        "https://github.com/kubernetes-csi/external-resizer",
	"mirrored-sig-storage-livenessprobe":                      "https://github.com/kubernetes-csi/livenessprobe",
	"mirrored-sig-storage-csi-snapshotter":                    "https://github.com/kubernetes-csi/external-snapshotter",
	"mirrored-sig-storage-snapshot-controller":                "https://github.com/kubernetes-csi/external-snapshotter",
	"mirrored-sig-storage-snapshot-validation-webhook":        "https://github.com/kubernetes-csi/external-snapshotter",
	"mirrored-sigwindowstools-k8s-gmsa-webhook":               "https://github.com/kubernetes-sigs/windows-gmsa/tree/master/admission-webhook",
	"mirrored-sonobuoy-sonobuoy":                              "https://github.com/vmware-tanzu/sonobuoy",
	"mirrored-thanos-thanos":                                  "https://github.com/thanos-io/thanos",
	"mirrored-thanosio-thanos":                                "https://github.com/thanos-io/thanos",
	"nginx-ingress-controller":                                "https://github.com/rancher/ingress-nginx",
	"mirrored-skopeo-skopeo":                                  "https://github.com/containers/skopeo",
	"openvswitch":                                             "https://github.com/servicefractal/ovs",
	"opflex":                                                  "https://github.com/noironetworks/opflex",
	"opflex-server":                                           "https://github.com/noironetworks/opflex",
	"pause":                                                   "https://hub.docker.com/r/kubernetes/pause",
	"prom-alertmanager":                                       "https://github.com/prometheus/alertmanager",
	"prom-node-exporter":                                      "https://github.com/prometheus/node_exporter",
	"prom-prometheus":                                         "https://github.com/prometheus/prometheus",
	"prometheus-auth":                                         "https://github.com/rancher/prometheus-auth",
	"prometheus-federator":                                    "https://github.com/rancher/prometheus-federator",
	"pushprox-client":                                         "https://github.com/rancher/PushProx",
	"pushprox-proxy":                                          "https://github.com/rancher/PushProx",
	"rancher":                                                 "https://github.com/rancher/rancher",
	"rancher-agent":                                           "https://github.com/rancher/rancher-agent",
	"rancher-csp-adapter":                                     "https://github.com/rancher/csp-adapter",
	"rancher-webhook":                                         "https://github.com/rancher/webhook",
	"rke-tools":                                               "https://github.com/rancher/rke-tools",
	"rke2-cloud-provider":                                     "https://github.com/rancher/image-build-rke2-cloud-provider",
	"rke2-runtime":                                            "https://github.com/rancher/rke2",
	"rke2-upgrade":                                            "https://github.com/rancher/rke2-upgrade",
	"security-scan":                                           "https://github.com/rancher/security-scan",
	"shell":                                                   "https://github.com/rancher/shell",
	"system-agent":                                            "https://github.com/rancher/system-agent",
	"system-agent-installer-k3s":                              "https://github.com/rancher/system-agent-installer-k3s",
	"system-agent-installer-rke2":                             "https://github.com/rancher/system-agent-installer-rke2",
	"system-upgrade-controller":                               "https://github.com/rancher/system-upgrade-controller",
	"tekton-utils":                                            "https://github.com/rancher/build-tekton",
	"thanosio-thanos":                                         "https://github.com/thanos-io/thanos",
	"ui-plugin-operator":                                      "https://github.com/rancher/ui-plugin-operator",
	"ui-plugin-catalog":                                       "https://github.com/rancher/ui-plugin-charts",
	"weave-kube":                                              "https://github.com/weaveworks-experiments/weave-kube",
	"weave-npc":                                               "https://github.com/weaveworks/weave",
	"webhook-receiver":                                        "https://github.com/rancher/webhook-receiver",
	"windows_exporter-package":                                "https://github.com/rancher/windows_exporter-package",
	"wins":                                                    "https://github.com/rancher/wins",
	"wmi_exporter-package":                                    "https://github.com/rancher/wmi_exporter-package",
}

const (
	imageOriginFileName = "rancher-images-origins.txt"
)

// GenerateImageOrigins looks through all target images gathered from KDM/Charts and
// ensures that they are covered by the OriginMap. If so, a file titled `rancher-image-origins.txt` will be written,
// containing a space delimited list of un-versioned  images and their source code repository.
func GenerateImageOrigins(linuxImagesFromArgs, targetImages, targetWindowsImages []string) error {
	log.Printf("building %s", imageOriginFileName)

	allImages := GetAllUniqueImages(linuxImagesFromArgs, targetImages, targetWindowsImages)

	// if any image does not have a source
	// declared, or if a net-new image has been
	// added via KDM or charts and is not
	// included in the OriginMap
	// then we cannot continue as the
	// artifact would be incomplete.
	if UnknownImages := GatherUnknownImages(allImages); len(UnknownImages) > 0 {
		return fmt.Errorf("[ERROR] not all images have a source code origin defined. Please provide origin URL's within rancher/pkg/image/origins.go for the following images: \n%s", UnknownImages)
	}

	// format file
	fileContents := ""
	for k, v := range OriginMap {
		fileContents = fileContents + k + " " + v + "\n"
	}

	originsFile, err := os.Create(imageOriginFileName)
	if err != nil {
		return fmt.Errorf("could not create %s file: %w", imageOriginFileName, err)
	}

	originsFile.Chmod(0755)
	originsFile.WriteString(fileContents)
	return originsFile.Close()
}

// GetAllUniqueImages looks through all the images gathered from KDM/Charts
// and returns a list of unique images, ignoring the image version
func GetAllUniqueImages(images ...[]string) []string {
	var allImages []string
	for _, imagesSlice := range images {
		allImages = append(allImages, UniqueTargetImages(imagesSlice)...)
	}
	return allImages
}

// GatherUnknownImages compares all images gathered from KDM and charts
// against the current OriginMap and finds repositories which are not currently
// included in the OriginMap.
func GatherUnknownImages(allImages []string) string {
	// compare all known images with images gathered
	// from KDM / Charts to detect images missing from OriginsMap.
	var unknownImages string
	for _, e := range allImages {
		url, ok := OriginMap[e]
		if !ok || url == "" || url == "unknown" {
			unknownImages = unknownImages + e + "\n"
		}
	}
	return unknownImages
}

// repoFromImage strips away the repository and version
// of a given image.
func repoFromImage(image string) string {
	split := strings.Split(image, "/")
	if len(split) != 2 {
		return ""
	}

	split = strings.Split(split[1], ":")
	if len(split) != 2 {
		return ""
	}

	return split[0]
}

// UniqueTargetImages finds unique images in a list of targetImages,
// ignoring the image version
func UniqueTargetImages(targetImages []string) []string {
	seenImages := make(map[string]interface{})
	var uniqueImages []string
	for _, e := range targetImages {
		repo := repoFromImage(e)
		_, ok := seenImages[repo]
		if !ok && repo != "" {
			seenImages[repo] = true
			uniqueImages = append(uniqueImages, repo)
		}
	}
	return uniqueImages
}
