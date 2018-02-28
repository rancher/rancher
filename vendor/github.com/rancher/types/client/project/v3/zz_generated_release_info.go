package client

const (
	ReleaseInfoType                   = "releaseInfo"
	ReleaseInfoFieldCreateTimestamp   = "createTimestamp"
	ReleaseInfoFieldModifiedAt        = "modifiedAt"
	ReleaseInfoFieldName              = "name"
	ReleaseInfoFieldTemplateVersionID = "templateVersionId"
	ReleaseInfoFieldVersion           = "version"
)

type ReleaseInfo struct {
	CreateTimestamp   string `json:"createTimestamp,omitempty" yaml:"createTimestamp,omitempty"`
	ModifiedAt        string `json:"modifiedAt,omitempty" yaml:"modifiedAt,omitempty"`
	Name              string `json:"name,omitempty" yaml:"name,omitempty"`
	TemplateVersionID string `json:"templateVersionId,omitempty" yaml:"templateVersionId,omitempty"`
	Version           string `json:"version,omitempty" yaml:"version,omitempty"`
}
