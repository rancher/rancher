package client

const (
	MachineConfigType                      = "machineConfig"
	MachineConfigFieldAnnotations          = "annotations"
	MachineConfigFieldCustomConfig         = "customConfig"
	MachineConfigFieldDescription          = "description"
	MachineConfigFieldDisplayName          = "displayName"
	MachineConfigFieldLabels               = "labels"
	MachineConfigFieldMachineTemplateId    = "machineTemplateId"
	MachineConfigFieldNodeSpec             = "nodeSpec"
	MachineConfigFieldRequestedHostname    = "requestedHostname"
	MachineConfigFieldRole                 = "role"
	MachineConfigFieldUseInternalIPAddress = "useInternalIpAddress"
)

type MachineConfig struct {
	Annotations          map[string]string `json:"annotations,omitempty"`
	CustomConfig         *CustomConfig     `json:"customConfig,omitempty"`
	Description          string            `json:"description,omitempty"`
	DisplayName          string            `json:"displayName,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	MachineTemplateId    string            `json:"machineTemplateId,omitempty"`
	NodeSpec             *NodeSpec         `json:"nodeSpec,omitempty"`
	RequestedHostname    string            `json:"requestedHostname,omitempty"`
	Role                 []string          `json:"role,omitempty"`
	UseInternalIPAddress *bool             `json:"useInternalIpAddress,omitempty"`
}
