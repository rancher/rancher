package client

const (
	TemplateVersionStatusType             = "templateVersionStatus"
	TemplateVersionStatusFieldHelmVersion = "helmVersion"
)

type TemplateVersionStatus struct {
	HelmVersion string `json:"helmVersion,omitempty" yaml:"helmVersion,omitempty"`
}
