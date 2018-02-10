package client

const (
	KubernetesInfoType                  = "kubernetesInfo"
	KubernetesInfoFieldKubeProxyVersion = "kubeProxyVersion"
	KubernetesInfoFieldKubeletVersion   = "kubeletVersion"
)

type KubernetesInfo struct {
	KubeProxyVersion string `json:"kubeProxyVersion,omitempty"`
	KubeletVersion   string `json:"kubeletVersion,omitempty"`
}
