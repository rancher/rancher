package client

const (
	RKESystemImagesType                           = "rkeSystemImages"
	RKESystemImagesFieldAciCniDeployContainer     = "aciCniDeployContainer"
	RKESystemImagesFieldAciControllerContainer    = "aciControllerContainer"
	RKESystemImagesFieldAciGbpServerContainer     = "aciGbpServerContainer"
	RKESystemImagesFieldAciHostContainer          = "aciHostContainer"
	RKESystemImagesFieldAciMcastContainer         = "aciMcastContainer"
	RKESystemImagesFieldAciOpenvSwitchContainer   = "aciOvsContainer"
	RKESystemImagesFieldAciOpflexContainer        = "aciOpflexContainer"
	RKESystemImagesFieldAciOpflexServerContainer  = "aciOpflexServerContainer"
	RKESystemImagesFieldAlpine                    = "alpine"
	RKESystemImagesFieldCalicoCNI                 = "calicoCni"
	RKESystemImagesFieldCalicoControllers         = "calicoControllers"
	RKESystemImagesFieldCalicoCtl                 = "calicoCtl"
	RKESystemImagesFieldCalicoFlexVol             = "calicoFlexVol"
	RKESystemImagesFieldCalicoNode                = "calicoNode"
	RKESystemImagesFieldCanalCNI                  = "canalCni"
	RKESystemImagesFieldCanalControllers          = "canalControllers"
	RKESystemImagesFieldCanalFlannel              = "canalFlannel"
	RKESystemImagesFieldCanalFlexVol              = "canalFlexVol"
	RKESystemImagesFieldCanalNode                 = "canalNode"
	RKESystemImagesFieldCertDownloader            = "certDownloader"
	RKESystemImagesFieldCoreDNS                   = "coredns"
	RKESystemImagesFieldCoreDNSAutoscaler         = "corednsAutoscaler"
	RKESystemImagesFieldDNSmasq                   = "dnsmasq"
	RKESystemImagesFieldEtcd                      = "etcd"
	RKESystemImagesFieldFlannel                   = "flannel"
	RKESystemImagesFieldFlannelCNI                = "flannelCni"
	RKESystemImagesFieldIngress                   = "ingress"
	RKESystemImagesFieldIngressBackend            = "ingressBackend"
	RKESystemImagesFieldIngressWebhook            = "ingressWebhook"
	RKESystemImagesFieldKubeDNS                   = "kubedns"
	RKESystemImagesFieldKubeDNSAutoscaler         = "kubednsAutoscaler"
	RKESystemImagesFieldKubeDNSSidecar            = "kubednsSidecar"
	RKESystemImagesFieldKubernetes                = "kubernetes"
	RKESystemImagesFieldKubernetesServicesSidecar = "kubernetesServicesSidecar"
	RKESystemImagesFieldMetricsServer             = "metricsServer"
	RKESystemImagesFieldNginxProxy                = "nginxProxy"
	RKESystemImagesFieldNodelocal                 = "nodelocal"
	RKESystemImagesFieldPodInfraContainer         = "podInfraContainer"
	RKESystemImagesFieldWeaveCNI                  = "weaveCni"
	RKESystemImagesFieldWeaveNode                 = "weaveNode"
	RKESystemImagesFieldWindowsPodInfraContainer  = "windowsPodInfraContainer"
)

type RKESystemImages struct {
	AciCniDeployContainer     string `json:"aciCniDeployContainer,omitempty" yaml:"aciCniDeployContainer,omitempty"`
	AciControllerContainer    string `json:"aciControllerContainer,omitempty" yaml:"aciControllerContainer,omitempty"`
	AciGbpServerContainer     string `json:"aciGbpServerContainer,omitempty" yaml:"aciGbpServerContainer,omitempty"`
	AciHostContainer          string `json:"aciHostContainer,omitempty" yaml:"aciHostContainer,omitempty"`
	AciMcastContainer         string `json:"aciMcastContainer,omitempty" yaml:"aciMcastContainer,omitempty"`
	AciOpenvSwitchContainer   string `json:"aciOvsContainer,omitempty" yaml:"aciOvsContainer,omitempty"`
	AciOpflexContainer        string `json:"aciOpflexContainer,omitempty" yaml:"aciOpflexContainer,omitempty"`
	AciOpflexServerContainer  string `json:"aciOpflexServerContainer,omitempty" yaml:"aciOpflexServerContainer,omitempty"`
	Alpine                    string `json:"alpine,omitempty" yaml:"alpine,omitempty"`
	CalicoCNI                 string `json:"calicoCni,omitempty" yaml:"calicoCni,omitempty"`
	CalicoControllers         string `json:"calicoControllers,omitempty" yaml:"calicoControllers,omitempty"`
	CalicoCtl                 string `json:"calicoCtl,omitempty" yaml:"calicoCtl,omitempty"`
	CalicoFlexVol             string `json:"calicoFlexVol,omitempty" yaml:"calicoFlexVol,omitempty"`
	CalicoNode                string `json:"calicoNode,omitempty" yaml:"calicoNode,omitempty"`
	CanalCNI                  string `json:"canalCni,omitempty" yaml:"canalCni,omitempty"`
	CanalControllers          string `json:"canalControllers,omitempty" yaml:"canalControllers,omitempty"`
	CanalFlannel              string `json:"canalFlannel,omitempty" yaml:"canalFlannel,omitempty"`
	CanalFlexVol              string `json:"canalFlexVol,omitempty" yaml:"canalFlexVol,omitempty"`
	CanalNode                 string `json:"canalNode,omitempty" yaml:"canalNode,omitempty"`
	CertDownloader            string `json:"certDownloader,omitempty" yaml:"certDownloader,omitempty"`
	CoreDNS                   string `json:"coredns,omitempty" yaml:"coredns,omitempty"`
	CoreDNSAutoscaler         string `json:"corednsAutoscaler,omitempty" yaml:"corednsAutoscaler,omitempty"`
	DNSmasq                   string `json:"dnsmasq,omitempty" yaml:"dnsmasq,omitempty"`
	Etcd                      string `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Flannel                   string `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	FlannelCNI                string `json:"flannelCni,omitempty" yaml:"flannelCni,omitempty"`
	Ingress                   string `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	IngressBackend            string `json:"ingressBackend,omitempty" yaml:"ingressBackend,omitempty"`
	IngressWebhook            string `json:"ingressWebhook,omitempty" yaml:"ingressWebhook,omitempty"`
	KubeDNS                   string `json:"kubedns,omitempty" yaml:"kubedns,omitempty"`
	KubeDNSAutoscaler         string `json:"kubednsAutoscaler,omitempty" yaml:"kubednsAutoscaler,omitempty"`
	KubeDNSSidecar            string `json:"kubednsSidecar,omitempty" yaml:"kubednsSidecar,omitempty"`
	Kubernetes                string `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	KubernetesServicesSidecar string `json:"kubernetesServicesSidecar,omitempty" yaml:"kubernetesServicesSidecar,omitempty"`
	MetricsServer             string `json:"metricsServer,omitempty" yaml:"metricsServer,omitempty"`
	NginxProxy                string `json:"nginxProxy,omitempty" yaml:"nginxProxy,omitempty"`
	Nodelocal                 string `json:"nodelocal,omitempty" yaml:"nodelocal,omitempty"`
	PodInfraContainer         string `json:"podInfraContainer,omitempty" yaml:"podInfraContainer,omitempty"`
	WeaveCNI                  string `json:"weaveCni,omitempty" yaml:"weaveCni,omitempty"`
	WeaveNode                 string `json:"weaveNode,omitempty" yaml:"weaveNode,omitempty"`
	WindowsPodInfraContainer  string `json:"windowsPodInfraContainer,omitempty" yaml:"windowsPodInfraContainer,omitempty"`
}
