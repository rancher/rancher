package v3

type WindowsSystemImages struct {
	// Windows nginx-proxy image
	NginxProxy string `yaml:"nginx_proxy" json:"nginxProxy,omitempty"`
	// Kubernetes binaries image
	KubernetesBinaries string `yaml:"kubernetes_binaries" json:"kubernetesBinaries,omitempty"`
	// Kubelet pause image
	KubeletPause string `yaml:"kubelet_pause" json:"kubeletPause,omitempty"`
	// Flannel CNI binaries image
	FlannelCNIBinaries string `yaml:"flannel_cni_binaries" json:"flannelCniBinaries,omitempty"`
	// Calico CNI binaries image
	CalicoCNIBinaries string `yaml:"calico_cni_binaries" json:"calicoCniBinaries,omitempty"`
	// Canal CNI binaries image
	CanalCNIBinaries string `yaml:"canal_cni_binaries" json:"canalCniBinaries,omitempty"`
}
