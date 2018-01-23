package client

const (
	MachineSpecType                      = "machineSpec"
	MachineSpecFieldClusterId            = "clusterId"
	MachineSpecFieldCustomConfig         = "customConfig"
	MachineSpecFieldDescription          = "description"
	MachineSpecFieldDisplayName          = "displayName"
	MachineSpecFieldMachineTemplateId    = "machineTemplateId"
	MachineSpecFieldPodCidr              = "podCidr"
	MachineSpecFieldProviderId           = "providerId"
	MachineSpecFieldRequestedHostname    = "requestedHostname"
	MachineSpecFieldRole                 = "role"
	MachineSpecFieldTaints               = "taints"
	MachineSpecFieldUnschedulable        = "unschedulable"
	MachineSpecFieldUseInternalIPAddress = "useInternalIpAddress"
)

type MachineSpec struct {
	ClusterId            string        `json:"clusterId,omitempty"`
	CustomConfig         *CustomConfig `json:"customConfig,omitempty"`
	Description          string        `json:"description,omitempty"`
	DisplayName          string        `json:"displayName,omitempty"`
	MachineTemplateId    string        `json:"machineTemplateId,omitempty"`
	PodCidr              string        `json:"podCidr,omitempty"`
	ProviderId           string        `json:"providerId,omitempty"`
	RequestedHostname    string        `json:"requestedHostname,omitempty"`
	Role                 []string      `json:"role,omitempty"`
	Taints               []Taint       `json:"taints,omitempty"`
	Unschedulable        *bool         `json:"unschedulable,omitempty"`
	UseInternalIPAddress *bool         `json:"useInternalIpAddress,omitempty"`
}
