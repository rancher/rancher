package client

const (
	NamespaceComposeSpecType                  = "namespaceComposeSpec"
	NamespaceComposeSpecFieldInstallNamespace = "installNamespace"
	NamespaceComposeSpecFieldProjectId        = "projectId"
	NamespaceComposeSpecFieldRancherCompose   = "rancherCompose"
)

type NamespaceComposeSpec struct {
	InstallNamespace string `json:"installNamespace,omitempty" yaml:"installNamespace,omitempty"`
	ProjectId        string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	RancherCompose   string `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
}
