package kdm

import (
	"encoding/json"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	Calico        = "calico"
	Canal         = "canal"
	Flannel       = "flannel"
	Weave         = "weave"
	CoreDNS       = "coreDNS"
	KubeDNS       = "kubeDNS"
	MetricsServer = "metricsServer"
	NginxIngress  = "nginxIngress"
	Nodelocal     = "nodelocal"
	TemplateKeys  = "templateKeys"
)

type Data struct {
	// K8sVersionServiceOptions - service options per k8s version
	K8sVersionServiceOptions  map[string]v3.KubernetesServicesOptions
	K8sVersionRKESystemImages map[string]v3.RKESystemImages

	// Addon Templates per K8s version ("default" where nothing changes for k8s version)
	K8sVersionedTemplates map[string]map[string]string

	// K8sVersionInfo - min/max RKE+Rancher versions per k8s version
	K8sVersionInfo map[string]v3.K8sVersionInfo

	//Default K8s version for every rancher version
	RancherDefaultK8sVersions map[string]string

	//Default K8s version for every rke version
	RKEDefaultK8sVersions map[string]string

	K8sVersionDockerInfo map[string][]string

	// K8sVersionWindowsServiceOptions - service options per windows k8s version
	K8sVersionWindowsServiceOptions map[string]v3.KubernetesServicesOptions

	CisConfigParams         map[string]v3.CisConfigParams
	CisBenchmarkVersionInfo map[string]v3.CisBenchmarkVersionInfo

	// K3S specific data, opaque and defined by the config file in kdm
	K3S map[string]interface{} `json:"k3s,omitempty"`
}

func FromData(b []byte) (Data, error) {
	d := &Data{}

	if err := json.Unmarshal(b, d); err != nil {
		return Data{}, err
	}
	return *d, nil
}
