package client

const (
	MachineSpecType                   = "machineSpec"
	MachineSpecFieldBootstrap         = "bootstrap"
	MachineSpecFieldClusterName       = "clusterName"
	MachineSpecFieldFailureDomain     = "failureDomain"
	MachineSpecFieldInfrastructureRef = "infrastructureRef"
	MachineSpecFieldNodeDrainTimeout  = "nodeDrainTimeout"
	MachineSpecFieldProviderID        = "providerID"
	MachineSpecFieldVersion           = "version"
)

type MachineSpec struct {
	Bootstrap         *Bootstrap       `json:"bootstrap,omitempty" yaml:"bootstrap,omitempty"`
	ClusterName       string           `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	FailureDomain     string           `json:"failureDomain,omitempty" yaml:"failureDomain,omitempty"`
	InfrastructureRef *ObjectReference `json:"infrastructureRef,omitempty" yaml:"infrastructureRef,omitempty"`
	NodeDrainTimeout  *Duration        `json:"nodeDrainTimeout,omitempty" yaml:"nodeDrainTimeout,omitempty"`
	ProviderID        string           `json:"providerID,omitempty" yaml:"providerID,omitempty"`
	Version           string           `json:"version,omitempty" yaml:"version,omitempty"`
}
