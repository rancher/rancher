package client

const (
	SaveAsTemplateInputType                             = "saveAsTemplateInput"
	SaveAsTemplateInputFieldClusterTemplateName         = "clusterTemplateName"
	SaveAsTemplateInputFieldClusterTemplateRevisionName = "clusterTemplateRevisionName"
)

type SaveAsTemplateInput struct {
	ClusterTemplateName         string `json:"clusterTemplateName,omitempty" yaml:"clusterTemplateName,omitempty"`
	ClusterTemplateRevisionName string `json:"clusterTemplateRevisionName,omitempty" yaml:"clusterTemplateRevisionName,omitempty"`
}
