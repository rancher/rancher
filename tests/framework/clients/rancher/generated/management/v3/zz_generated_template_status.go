package client

const (
	TemplateStatusType             = "templateStatus"
	TemplateStatusFieldHelmVersion = "helmVersion"
)

type TemplateStatus struct {
	HelmVersion string `json:"helmVersion,omitempty" yaml:"helmVersion,omitempty"`
}
