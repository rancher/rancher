package client

const (
	AzureFileVolumeSourceType            = "azureFileVolumeSource"
	AzureFileVolumeSourceFieldReadOnly   = "readOnly"
	AzureFileVolumeSourceFieldSecretName = "secretName"
	AzureFileVolumeSourceFieldShareName  = "shareName"
)

type AzureFileVolumeSource struct {
	ReadOnly   *bool  `json:"readOnly,omitempty"`
	SecretName string `json:"secretName,omitempty"`
	ShareName  string `json:"shareName,omitempty"`
}
