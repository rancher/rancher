package client

const (
	NodeDriverSpecType             = "nodeDriverSpec"
	NodeDriverSpecFieldActive      = "active"
	NodeDriverSpecFieldBuiltin     = "builtin"
	NodeDriverSpecFieldChecksum    = "checksum"
	NodeDriverSpecFieldDescription = "description"
	NodeDriverSpecFieldDisplayName = "displayName"
	NodeDriverSpecFieldExternalID  = "externalId"
	NodeDriverSpecFieldUIURL       = "uiUrl"
	NodeDriverSpecFieldURL         = "url"
)

type NodeDriverSpec struct {
	Active      bool   `json:"active,omitempty"`
	Builtin     bool   `json:"builtin,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Description string `json:"description,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	ExternalID  string `json:"externalId,omitempty"`
	UIURL       string `json:"uiUrl,omitempty"`
	URL         string `json:"url,omitempty"`
}
