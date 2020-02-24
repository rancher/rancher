package client

const (
	CisBenchmarkVersionInfoType                      = "cisBenchmarkVersionInfo"
	CisBenchmarkVersionInfoFieldMinKubernetesVersion = "minKubernetesVersion"
)

type CisBenchmarkVersionInfo struct {
	MinKubernetesVersion string `json:"minKubernetesVersion,omitempty" yaml:"minKubernetesVersion,omitempty"`
}
