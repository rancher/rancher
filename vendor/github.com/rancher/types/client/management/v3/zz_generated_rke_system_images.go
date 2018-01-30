package client

const (
	RKESystemImagesType                           = "rkeSystemImages"
	RKESystemImagesFieldAlpine                    = "alpine"
	RKESystemImagesFieldCertDownloader            = "certDownloader"
	RKESystemImagesFieldDNSmasq                   = "dnsmasq"
	RKESystemImagesFieldEtcd                      = "etcd"
	RKESystemImagesFieldKubeDNS                   = "kubedns"
	RKESystemImagesFieldKubeDNSAutoscaler         = "kubednsAutoscaler"
	RKESystemImagesFieldKubeDNSSidecar            = "kubednsSidecar"
	RKESystemImagesFieldKubernetes                = "kubernetes"
	RKESystemImagesFieldKubernetesServicesSidecar = "kubernetesServicesSidecar"
	RKESystemImagesFieldNginxProxy                = "nginxProxy"
)

type RKESystemImages struct {
	Alpine                    string `json:"alpine,omitempty"`
	CertDownloader            string `json:"certDownloader,omitempty"`
	DNSmasq                   string `json:"dnsmasq,omitempty"`
	Etcd                      string `json:"etcd,omitempty"`
	KubeDNS                   string `json:"kubedns,omitempty"`
	KubeDNSAutoscaler         string `json:"kubednsAutoscaler,omitempty"`
	KubeDNSSidecar            string `json:"kubednsSidecar,omitempty"`
	Kubernetes                string `json:"kubernetes,omitempty"`
	KubernetesServicesSidecar string `json:"kubernetesServicesSidecar,omitempty"`
	NginxProxy                string `json:"nginxProxy,omitempty"`
}
