package rancherversion

const (
	ConfigurationFileKey = "prime"
)

type Config struct {
	RancherVersion string `json:"rancherVersion" yaml:"rancherVersion"`
	IsPrime        bool   `json:"isPrime" yaml:"isPrime" default:false`
	GitCommit      string `json:"gitCommit" yaml:"gitCommit"`
}
