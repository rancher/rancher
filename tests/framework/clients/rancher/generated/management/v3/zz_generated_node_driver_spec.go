package client

const (
	NodeDriverSpecType                    = "nodeDriverSpec"
	NodeDriverSpecFieldActive             = "active"
	NodeDriverSpecFieldAddCloudCredential = "addCloudCredential"
	NodeDriverSpecFieldBuiltin            = "builtin"
	NodeDriverSpecFieldChecksum           = "checksum"
	NodeDriverSpecFieldDescription        = "description"
	NodeDriverSpecFieldDisplayName        = "displayName"
	NodeDriverSpecFieldExternalID         = "externalId"
	NodeDriverSpecFieldUIURL              = "uiUrl"
	NodeDriverSpecFieldURL                = "url"
	NodeDriverSpecFieldWhitelistDomains   = "whitelistDomains"
)

type NodeDriverSpec struct {
	Active             bool     `json:"active,omitempty" yaml:"active,omitempty"`
	AddCloudCredential bool     `json:"addCloudCredential,omitempty" yaml:"addCloudCredential,omitempty"`
	Builtin            bool     `json:"builtin,omitempty" yaml:"builtin,omitempty"`
	Checksum           string   `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	Description        string   `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName        string   `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ExternalID         string   `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	UIURL              string   `json:"uiUrl,omitempty" yaml:"uiUrl,omitempty"`
	URL                string   `json:"url,omitempty" yaml:"url,omitempty"`
	WhitelistDomains   []string `json:"whitelistDomains,omitempty" yaml:"whitelistDomains,omitempty"`
}
