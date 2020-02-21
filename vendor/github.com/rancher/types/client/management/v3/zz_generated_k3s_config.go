package client

const (
	K3sConfigType                   = "k3sConfig"
	K3sConfigFieldServerConcurrency = "serverConcurrency"
	K3sConfigFieldVersion           = "kubernetesVersion"
	K3sConfigFieldWorkerConcurrency = "workerConcurrency"
)

type K3sConfig struct {
	ServerConcurrency int64 `json:"serverConcurrency,omitempty" yaml:"serverConcurrency,omitempty"`
	Version           *Info `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	WorkerConcurrency int64 `json:"workerConcurrency,omitempty" yaml:"workerConcurrency,omitempty"`
}
