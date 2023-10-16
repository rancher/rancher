package rancherversion

const (
	ConfigurationFileKey = "prime"
)

type Config struct {
	Brand          string `json:"brand" yaml:"brand"`
	GitCommit      string `json:"gitCommit" yaml:"gitCommit"`
	IsPrime        bool   `json:"isPrime" yaml:"isPrime" default:"false"`
	RancherVersion string `json:"rancherVersion" yaml:"rancherVersion"`
	Registry       string `json:"registry" yaml:"registry"`
}
