package client

const (
	AzureFileVolumeSourceType            = "azureFileVolumeSource"
	AzureFileVolumeSourceFieldReadOnly   = "readOnly"
	AzureFileVolumeSourceFieldSecretName = "secretName"
	AzureFileVolumeSourceFieldShareName  = "shareName"
)

type AzureFileVolumeSource struct {
	ReadOnly   bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretName string `json:"secretName,omitempty" yaml:"secretName,omitempty"`
	ShareName  string `json:"shareName,omitempty" yaml:"shareName,omitempty"`
}
