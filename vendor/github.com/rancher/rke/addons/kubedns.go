package addons

import "github.com/rancher/rke/templates"

const (
	KubeDNSImage           = "KubeDNSImage"
	DNSMasqImage           = "DNSMasqImage"
	KubeDNSSidecarImage    = "KubednsSidecarImage"
	KubeDNSAutoScalerImage = "KubeDNSAutoScalerImage"
	KubeDNSServer          = "ClusterDNSServer"
	KubeDNSClusterDomain   = "ClusterDomain"
	RBAC                   = "RBAC"
)

func GetKubeDNSManifest(kubeDNSConfig map[string]string) (string, error) {

	return templates.CompileTemplateFromMap(templates.KubeDNSTemplate, kubeDNSConfig)
}
