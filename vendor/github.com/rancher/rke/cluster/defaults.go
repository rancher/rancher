package cluster

const (
	DefaultClusterConfig = "cluster.yml"

	DefaultServiceClusterIPRange = "10.233.0.0/18"
	DefaultClusterCIDR           = "10.233.64.0/18"
	DefaultClusterDNSService     = "10.233.0.3"
	DefaultClusterDomain         = "cluster.local"
	DefaultClusterSSHKeyPath     = "~/.ssh/id_rsa"

	DefaultDockerSockPath = "/var/run/docker.sock"

	DefaultAuthStrategy      = "x509"
	DefaultAuthorizationMode = "rbac"

	DefaultNetworkPlugin        = "flannel"
	DefaultNetworkCloudProvider = "none"

	DefaultInfraContainerImage  = "gcr.io/google_containers/pause-amd64:3.0"
	DefaultAplineImage          = "alpine:latest"
	DefaultNginxProxyImage      = "rancher/rke-nginx-proxy:v0.1.0"
	DefaultCertDownloaderImage  = "rancher/rke-cert-deployer:v0.1.1"
	DefaultServiceSidekickImage = "rancher/rke-service-sidekick:v0.1.0"

	DefaultEtcdImage = "quay.io/coreos/etcd:latest"
	DefaultK8sImage  = "rancher/k8s:v1.8.3-rancher2"

	DefaultFlannelImage    = "quay.io/coreos/flannel:v0.9.1"
	DefaultFlannelCNIImage = "quay.io/coreos/flannel-cni:v0.2.0"

	DefaultCalicoNodeImage        = "quay.io/calico/node:v2.6.2"
	DefaultCalicoCNIImage         = "quay.io/calico/cni:v1.11.0"
	DefaultCalicoControllersImage = "quay.io/calico/kube-controllers:v1.0.0"
	DefaultCalicoctlImage         = "quay.io/calico/ctl:v1.6.2"

	DefaultWeaveImage    = "weaveworks/weave-kube:2.1.2"
	DefaultWeaveCNIImage = "weaveworks/weave-npc:2.1.2"

	DefaultCanalNodeImage    = "quay.io/calico/node:v2.6.2"
	DefaultCanalCNIImage     = "quay.io/calico/cni:v1.11.0"
	DefaultCanalFlannelImage = "quay.io/coreos/flannel:v0.9.1"

	DefaultKubeDNSImage           = "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5"
	DefaultDNSMasqImage           = "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5"
	DefaultKubeDNSSidecarImage    = "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5"
	DefaultKubeDNSAutoScalerImage = "gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"
)

func setDefaultIfEmptyMapValue(configMap map[string]string, key string, value string) {
	if _, ok := configMap[key]; !ok {
		configMap[key] = value
	}
}
func setDefaultIfEmpty(varName *string, defaultValue string) {
	if len(*varName) == 0 {
		*varName = defaultValue
	}
}
