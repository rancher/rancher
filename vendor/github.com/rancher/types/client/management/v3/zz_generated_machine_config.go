package client

const (
	MachineConfigType                   = "machineConfig"
	MachineConfigFieldAnnotations       = "annotations"
	MachineConfigFieldDescription       = "description"
	MachineConfigFieldDisplayName       = "displayName"
	MachineConfigFieldLabels            = "labels"
	MachineConfigFieldMachineTemplateId = "machineTemplateId"
	MachineConfigFieldNodeSpec          = "nodeSpec"
	MachineConfigFieldRequestedHostname = "requestedHostname"
	MachineConfigFieldRole              = "role"
)

type MachineConfig struct {
	Annotations       map[string]string `json:"annotations,omitempty"`
	Description       string            `json:"description,omitempty"`
	DisplayName       string            `json:"displayName,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	MachineTemplateId string            `json:"machineTemplateId,omitempty"`
	NodeSpec          *NodeSpec         `json:"nodeSpec,omitempty"`
	RequestedHostname string            `json:"requestedHostname,omitempty"`
	Role              []string          `json:"role,omitempty"`
}
