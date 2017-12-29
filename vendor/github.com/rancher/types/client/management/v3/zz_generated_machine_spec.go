package client

const (
	MachineSpecType                    = "machineSpec"
	MachineSpecFieldDescription        = "description"
	MachineSpecFieldDisplayName        = "displayName"
	MachineSpecFieldMachineTemplateId  = "machineTemplateId"
	MachineSpecFieldPodCidr            = "podCidr"
	MachineSpecFieldProviderId         = "providerId"
	MachineSpecFieldRequestedClusterId = "requestedClusterId"
	MachineSpecFieldRequestedHostname  = "requestedHostname"
	MachineSpecFieldRequestedRoles     = "requestedRoles"
	MachineSpecFieldTaints             = "taints"
	MachineSpecFieldUnschedulable      = "unschedulable"
)

type MachineSpec struct {
	Description        string   `json:"description,omitempty"`
	DisplayName        string   `json:"displayName,omitempty"`
	MachineTemplateId  string   `json:"machineTemplateId,omitempty"`
	PodCidr            string   `json:"podCidr,omitempty"`
	ProviderId         string   `json:"providerId,omitempty"`
	RequestedClusterId string   `json:"requestedClusterId,omitempty"`
	RequestedHostname  string   `json:"requestedHostname,omitempty"`
	RequestedRoles     []string `json:"requestedRoles,omitempty"`
	Taints             []Taint  `json:"taints,omitempty"`
	Unschedulable      *bool    `json:"unschedulable,omitempty"`
}
