package client

const (
	MachineSpecType                         = "machineSpec"
	MachineSpecFieldBootstrap               = "bootstrap"
	MachineSpecFieldClusterName             = "clusterName"
	MachineSpecFieldFailureDomain           = "failureDomain"
	MachineSpecFieldInfrastructureRef       = "infrastructureRef"
	MachineSpecFieldNodeDeletionTimeout     = "nodeDeletionTimeout"
	MachineSpecFieldNodeDrainTimeout        = "nodeDrainTimeout"
	MachineSpecFieldNodeVolumeDetachTimeout = "nodeVolumeDetachTimeout"
	MachineSpecFieldProviderID              = "providerID"
	MachineSpecFieldVersion                 = "version"
)

type MachineSpec struct {
	Bootstrap               *Bootstrap       `json:"bootstrap,omitempty" yaml:"bootstrap,omitempty"`
	ClusterName             string           `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	FailureDomain           string           `json:"failureDomain,omitempty" yaml:"failureDomain,omitempty"`
	InfrastructureRef       *ObjectReference `json:"infrastructureRef,omitempty" yaml:"infrastructureRef,omitempty"`
	NodeDeletionTimeout     *Duration        `json:"nodeDeletionTimeout,omitempty" yaml:"nodeDeletionTimeout,omitempty"`
	NodeDrainTimeout        *Duration        `json:"nodeDrainTimeout,omitempty" yaml:"nodeDrainTimeout,omitempty"`
	NodeVolumeDetachTimeout *Duration        `json:"nodeVolumeDetachTimeout,omitempty" yaml:"nodeVolumeDetachTimeout,omitempty"`
	ProviderID              string           `json:"providerID,omitempty" yaml:"providerID,omitempty"`
	Version                 string           `json:"version,omitempty" yaml:"version,omitempty"`
}
