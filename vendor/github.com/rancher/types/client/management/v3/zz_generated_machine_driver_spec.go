package client

const (
	MachineDriverSpecType                  = "machineDriverSpec"
	MachineDriverSpecFieldActivateOnCreate = "activateOnCreate"
	MachineDriverSpecFieldBuiltin          = "builtin"
	MachineDriverSpecFieldChecksum         = "checksum"
	MachineDriverSpecFieldDefaultActive    = "defaultActive"
	MachineDriverSpecFieldDescription      = "description"
	MachineDriverSpecFieldDisplayName      = "displayName"
	MachineDriverSpecFieldExternalID       = "externalId"
	MachineDriverSpecFieldUIURL            = "uiUrl"
	MachineDriverSpecFieldURL              = "url"
)

type MachineDriverSpec struct {
	ActivateOnCreate *bool  `json:"activateOnCreate,omitempty"`
	Builtin          *bool  `json:"builtin,omitempty"`
	Checksum         string `json:"checksum,omitempty"`
	DefaultActive    *bool  `json:"defaultActive,omitempty"`
	Description      string `json:"description,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	ExternalID       string `json:"externalId,omitempty"`
	UIURL            string `json:"uiUrl,omitempty"`
	URL              string `json:"url,omitempty"`
}
