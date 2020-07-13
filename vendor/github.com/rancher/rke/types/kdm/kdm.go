package kdm

import (
	"encoding/json"

	v3 "github.com/rancher/rke/types"
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

// +k8s:deepcopy-gen=false

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

	CisConfigParams         map[string]CisConfigParams
	CisBenchmarkVersionInfo map[string]CisBenchmarkVersionInfo

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

type CisBenchmarkVersionInfo struct {
	Managed              bool              `yaml:"managed" json:"managed"`
	MinKubernetesVersion string            `yaml:"min_kubernetes_version" json:"minKubernetesVersion"`
	SkippedChecks        map[string]string `yaml:"skipped_checks" json:"skippedChecks"`
	NotApplicableChecks  map[string]string `yaml:"not_applicable_checks" json:"notApplicableChecks"`
}

type CisConfigParams struct {
	BenchmarkVersion string `yaml:"benchmark_version" json:"benchmarkVersion"`
}
