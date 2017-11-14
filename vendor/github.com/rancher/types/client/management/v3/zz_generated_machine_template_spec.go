package client

const (
	MachineTemplateSpecType              = "machineTemplateSpec"
	MachineTemplateSpecFieldDescription  = "description"
	MachineTemplateSpecFieldDisplayName  = "displayName"
	MachineTemplateSpecFieldDriver       = "driver"
	MachineTemplateSpecFieldFlavorPrefix = "flavorPrefix"
	MachineTemplateSpecFieldPublicValues = "publicValues"
	MachineTemplateSpecFieldSecretName   = "secretName"
	MachineTemplateSpecFieldSecretValues = "secretValues"
)

type MachineTemplateSpec struct {
	Description  string            `json:"description,omitempty"`
	DisplayName  string            `json:"displayName,omitempty"`
	Driver       string            `json:"driver,omitempty"`
	FlavorPrefix string            `json:"flavorPrefix,omitempty"`
	PublicValues map[string]string `json:"publicValues,omitempty"`
	SecretName   string            `json:"secretName,omitempty"`
	SecretValues map[string]string `json:"secretValues,omitempty"`
}
