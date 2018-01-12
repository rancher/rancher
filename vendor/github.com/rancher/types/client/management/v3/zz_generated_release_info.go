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
	CreateTimestamp   string `json:"createTimestamp,omitempty"`
	ModifiedAt        string `json:"modifiedAt,omitempty"`
	Name              string `json:"name,omitempty"`
	TemplateVersionID string `json:"templateVersionId,omitempty"`
	Version           string `json:"version,omitempty"`
}
