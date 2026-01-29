package client

const (
	MachineSpecType                   = "machineSpec"
	MachineSpecFieldBootstrap         = "bootstrap"
	MachineSpecFieldClusterName       = "clusterName"
	MachineSpecFieldDeletion          = "deletion"
	MachineSpecFieldFailureDomain     = "failureDomain"
	MachineSpecFieldInfrastructureRef = "infrastructureRef"
	MachineSpecFieldMinReadySeconds   = "minReadySeconds"
	MachineSpecFieldProviderID        = "providerID"
	MachineSpecFieldReadinessGates    = "readinessGates"
	MachineSpecFieldVersion           = "version"
)

type MachineSpec struct {
	Bootstrap         *Bootstrap                        `json:"bootstrap,omitempty" yaml:"bootstrap,omitempty"`
	ClusterName       string                            `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	Deletion          *MachineDeletionSpec              `json:"deletion,omitempty" yaml:"deletion,omitempty"`
	FailureDomain     string                            `json:"failureDomain,omitempty" yaml:"failureDomain,omitempty"`
	InfrastructureRef *ContractVersionedObjectReference `json:"infrastructureRef,omitempty" yaml:"infrastructureRef,omitempty"`
	MinReadySeconds   *int64                            `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	ProviderID        string                            `json:"providerID,omitempty" yaml:"providerID,omitempty"`
	ReadinessGates    []MachineReadinessGate            `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	Version           string                            `json:"version,omitempty" yaml:"version,omitempty"`
}
