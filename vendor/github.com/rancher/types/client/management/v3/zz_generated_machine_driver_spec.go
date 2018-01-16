package client

const (
	MachineDriverSpecType             = "machineDriverSpec"
	MachineDriverSpecFieldActive      = "active"
	MachineDriverSpecFieldBuiltin     = "builtin"
	MachineDriverSpecFieldChecksum    = "checksum"
	MachineDriverSpecFieldDescription = "description"
	MachineDriverSpecFieldDisplayName = "displayName"
	MachineDriverSpecFieldExternalID  = "externalId"
	MachineDriverSpecFieldUIURL       = "uiUrl"
	MachineDriverSpecFieldURL         = "url"
)

type MachineDriverSpec struct {
	Active      *bool  `json:"active,omitempty"`
	Builtin     *bool  `json:"builtin,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Description string `json:"description,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	ExternalID  string `json:"externalId,omitempty"`
	UIURL       string `json:"uiUrl,omitempty"`
	URL         string `json:"url,omitempty"`
}
