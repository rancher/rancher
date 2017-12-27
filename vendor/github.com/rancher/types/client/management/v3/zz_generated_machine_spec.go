package client

const (
	MachineSpecType                   = "machineSpec"
	MachineSpecFieldClusterId         = "clusterId"
	MachineSpecFieldConfigSource      = "configSource"
	MachineSpecFieldDescription       = "description"
	MachineSpecFieldExternalId        = "externalId"
	MachineSpecFieldMachineTemplateId = "machineTemplateId"
	MachineSpecFieldPodCIDR           = "podCIDR"
	MachineSpecFieldProviderID        = "providerID"
	MachineSpecFieldRoles             = "roles"
	MachineSpecFieldTaints            = "taints"
	MachineSpecFieldUnschedulable     = "unschedulable"
)

type MachineSpec struct {
	ClusterId         string            `json:"clusterId,omitempty"`
	ConfigSource      *NodeConfigSource `json:"configSource,omitempty"`
	Description       string            `json:"description,omitempty"`
	ExternalId        string            `json:"externalId,omitempty"`
	MachineTemplateId string            `json:"machineTemplateId,omitempty"`
	PodCIDR           string            `json:"podCIDR,omitempty"`
	ProviderID        string            `json:"providerID,omitempty"`
	Roles             []string          `json:"roles,omitempty"`
	Taints            []Taint           `json:"taints,omitempty"`
	Unschedulable     *bool             `json:"unschedulable,omitempty"`
}
