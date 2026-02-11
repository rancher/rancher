package client

const (
	MachineInitializationStatusType                            = "machineInitializationStatus"
	MachineInitializationStatusFieldBootstrapDataSecretCreated = "bootstrapDataSecretCreated"
	MachineInitializationStatusFieldInfrastructureProvisioned  = "infrastructureProvisioned"
)

type MachineInitializationStatus struct {
	BootstrapDataSecretCreated *bool `json:"bootstrapDataSecretCreated,omitempty" yaml:"bootstrapDataSecretCreated,omitempty"`
	InfrastructureProvisioned  *bool `json:"infrastructureProvisioned,omitempty" yaml:"infrastructureProvisioned,omitempty"`
}
