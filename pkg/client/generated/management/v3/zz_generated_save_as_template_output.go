package client

const (
	SaveAsTemplateOutputType                             = "saveAsTemplateOutput"
	SaveAsTemplateOutputFieldClusterTemplateName         = "clusterTemplateName"
	SaveAsTemplateOutputFieldClusterTemplateRevisionName = "clusterTemplateRevisionName"
)

type SaveAsTemplateOutput struct {
	ClusterTemplateName         string `json:"clusterTemplateName,omitempty" yaml:"clusterTemplateName,omitempty"`
	ClusterTemplateRevisionName string `json:"clusterTemplateRevisionName,omitempty" yaml:"clusterTemplateRevisionName,omitempty"`
}
