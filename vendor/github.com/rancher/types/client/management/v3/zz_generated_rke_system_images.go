package client

const (
	RKESystemImagesType                           = "rkeSystemImages"
	RKESystemImagesFieldAlpine                    = "alpine"
	RKESystemImagesFieldCalicoCNI                 = "calicoCni"
	RKESystemImagesFieldCalicoControllers         = "calicoControllers"
	RKESystemImagesFieldCalicoCtl                 = "calicoCtl"
	RKESystemImagesFieldCalicoNode                = "calicoNode"
	RKESystemImagesFieldCanalCNI                  = "canalCni"
	RKESystemImagesFieldCanalFlannel              = "canalFlannel"
	RKESystemImagesFieldCanalNode                 = "canalNode"
	RKESystemImagesFieldCertDownloader            = "certDownloader"
	RKESystemImagesFieldDNSmasq                   = "dnsmasq"
	RKESystemImagesFieldEtcd                      = "etcd"
	RKESystemImagesFieldFlannel                   = "flannel"
	RKESystemImagesFieldFlannelCNI                = "flannelCni"
	RKESystemImagesFieldKubeDNS                   = "kubedns"
	RKESystemImagesFieldKubeDNSAutoscaler         = "kubednsAutoscaler"
	RKESystemImagesFieldKubeDNSSidecar            = "kubednsSidecar"
	RKESystemImagesFieldKubernetes                = "kubernetes"
	RKESystemImagesFieldKubernetesServicesSidecar = "kubernetesServicesSidecar"
	RKESystemImagesFieldNginxProxy                = "nginxProxy"
	RKESystemImagesFieldPodInfraContainer         = "podInfraContainer"
	RKESystemImagesFieldWeaveCNI                  = "weaveCni"
	RKESystemImagesFieldWeaveNode                 = "weaveNode"
)

type RKESystemImages struct {
	Alpine                    string `json:"alpine,omitempty"`
	CalicoCNI                 string `json:"calicoCni,omitempty"`
	CalicoControllers         string `json:"calicoControllers,omitempty"`
	CalicoCtl                 string `json:"calicoCtl,omitempty"`
	CalicoNode                string `json:"calicoNode,omitempty"`
	CanalCNI                  string `json:"canalCni,omitempty"`
	CanalFlannel              string `json:"canalFlannel,omitempty"`
	CanalNode                 string `json:"canalNode,omitempty"`
	CertDownloader            string `json:"certDownloader,omitempty"`
	DNSmasq                   string `json:"dnsmasq,omitempty"`
	Etcd                      string `json:"etcd,omitempty"`
	Flannel                   string `json:"flannel,omitempty"`
	FlannelCNI                string `json:"flannelCni,omitempty"`
	KubeDNS                   string `json:"kubedns,omitempty"`
	KubeDNSAutoscaler         string `json:"kubednsAutoscaler,omitempty"`
	KubeDNSSidecar            string `json:"kubednsSidecar,omitempty"`
	Kubernetes                string `json:"kubernetes,omitempty"`
	KubernetesServicesSidecar string `json:"kubernetesServicesSidecar,omitempty"`
	NginxProxy                string `json:"nginxProxy,omitempty"`
	PodInfraContainer         string `json:"podInfraContainer,omitempty"`
	WeaveCNI                  string `json:"weaveCni,omitempty"`
	WeaveNode                 string `json:"weaveNode,omitempty"`
}
