package client

const (
	KubernetesInfoType                  = "kubernetesInfo"
	KubernetesInfoFieldKubeProxyVersion = "kubeProxyVersion"
	KubernetesInfoFieldKubeletVersion   = "kubeletVersion"
)

type KubernetesInfo struct {
	KubeProxyVersion string `json:"kubeProxyVersion,omitempty" yaml:"kubeProxyVersion,omitempty"`
	KubeletVersion   string `json:"kubeletVersion,omitempty" yaml:"kubeletVersion,omitempty"`
}
