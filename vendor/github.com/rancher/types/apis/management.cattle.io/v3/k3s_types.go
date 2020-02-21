package v3

import "k8s.io/apimachinery/pkg/version"

//K3sConfig provides desired configuration for k3s clusters
type K3sConfig struct {
	// k3s Kubernetes version
	Version *version.Info `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`
	K3sUpgradeStrategy
}

//K3sUpgradeStrategy provides configuration to the downstream system-upgrade-controller
type K3sUpgradeStrategy struct {
	// How many controlplane nodes should be upgrade at time, defaults to 1
	ServerConcurrency int `yaml:"server_concurrency" json:"serverConcurrency,omitempty"`
	// How many workers should be upgraded at a time
	WorkerConcurrency int `yaml:"worker_concurrency" json:"workerConcurrency,omitempty"`
}
