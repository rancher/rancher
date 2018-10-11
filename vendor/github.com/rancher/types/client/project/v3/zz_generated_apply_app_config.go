package client

const (
	ApplyAppConfigType                 = "applyAppConfig"
	ApplyAppConfigFieldAnswers         = "answers"
	ApplyAppConfigFieldCatalog         = "catalog"
	ApplyAppConfigFieldName            = "name"
	ApplyAppConfigFieldTargetNamespace = "targetNamespace"
	ApplyAppConfigFieldVersion         = "version"
)

type ApplyAppConfig struct {
	Answers         map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Catalog         string            `json:"catalog,omitempty" yaml:"catalog,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	TargetNamespace string            `json:"targetNamespace,omitempty" yaml:"targetNamespace,omitempty"`
	Version         string            `json:"version,omitempty" yaml:"version,omitempty"`
}
