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
	RKESystemImagesFieldIngress                   = "ingress"
	RKESystemImagesFieldIngressBackend            = "ingressBackend"
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
	Alpine                    string `json:"alpine,omitempty" yaml:"alpine,omitempty"`
	CalicoCNI                 string `json:"calicoCni,omitempty" yaml:"calicoCni,omitempty"`
	CalicoControllers         string `json:"calicoControllers,omitempty" yaml:"calicoControllers,omitempty"`
	CalicoCtl                 string `json:"calicoCtl,omitempty" yaml:"calicoCtl,omitempty"`
	CalicoNode                string `json:"calicoNode,omitempty" yaml:"calicoNode,omitempty"`
	CanalCNI                  string `json:"canalCni,omitempty" yaml:"canalCni,omitempty"`
	CanalFlannel              string `json:"canalFlannel,omitempty" yaml:"canalFlannel,omitempty"`
	CanalNode                 string `json:"canalNode,omitempty" yaml:"canalNode,omitempty"`
	CertDownloader            string `json:"certDownloader,omitempty" yaml:"certDownloader,omitempty"`
	DNSmasq                   string `json:"dnsmasq,omitempty" yaml:"dnsmasq,omitempty"`
	Etcd                      string `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Flannel                   string `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	FlannelCNI                string `json:"flannelCni,omitempty" yaml:"flannelCni,omitempty"`
	Ingress                   string `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	IngressBackend            string `json:"ingressBackend,omitempty" yaml:"ingressBackend,omitempty"`
	KubeDNS                   string `json:"kubedns,omitempty" yaml:"kubedns,omitempty"`
	KubeDNSAutoscaler         string `json:"kubednsAutoscaler,omitempty" yaml:"kubednsAutoscaler,omitempty"`
	KubeDNSSidecar            string `json:"kubednsSidecar,omitempty" yaml:"kubednsSidecar,omitempty"`
	Kubernetes                string `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	KubernetesServicesSidecar string `json:"kubernetesServicesSidecar,omitempty" yaml:"kubernetesServicesSidecar,omitempty"`
	NginxProxy                string `json:"nginxProxy,omitempty" yaml:"nginxProxy,omitempty"`
	PodInfraContainer         string `json:"podInfraContainer,omitempty" yaml:"podInfraContainer,omitempty"`
	WeaveCNI                  string `json:"weaveCni,omitempty" yaml:"weaveCni,omitempty"`
	WeaveNode                 string `json:"weaveNode,omitempty" yaml:"weaveNode,omitempty"`
}
