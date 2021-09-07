package client

const (
	AzureFilePersistentVolumeSourceType                 = "azureFilePersistentVolumeSource"
	AzureFilePersistentVolumeSourceFieldReadOnly        = "readOnly"
	AzureFilePersistentVolumeSourceFieldSecretName      = "secretName"
	AzureFilePersistentVolumeSourceFieldSecretNamespace = "secretNamespace"
	AzureFilePersistentVolumeSourceFieldShareName       = "shareName"
)

type AzureFilePersistentVolumeSource struct {
	ReadOnly        bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretName      string `json:"secretName,omitempty" yaml:"secretName,omitempty"`
	SecretNamespace string `json:"secretNamespace,omitempty" yaml:"secretNamespace,omitempty"`
	ShareName       string `json:"shareName,omitempty" yaml:"shareName,omitempty"`
}
