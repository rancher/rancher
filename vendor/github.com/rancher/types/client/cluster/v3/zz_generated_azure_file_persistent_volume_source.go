package client

const (
	AzureFilePersistentVolumeSourceType                 = "azureFilePersistentVolumeSource"
	AzureFilePersistentVolumeSourceFieldReadOnly        = "readOnly"
	AzureFilePersistentVolumeSourceFieldSecretName      = "secretName"
	AzureFilePersistentVolumeSourceFieldSecretNamespace = "secretNamespace"
	AzureFilePersistentVolumeSourceFieldShareName       = "shareName"
)

type AzureFilePersistentVolumeSource struct {
	ReadOnly        bool   `json:"readOnly,omitempty"`
	SecretName      string `json:"secretName,omitempty"`
	SecretNamespace string `json:"secretNamespace,omitempty"`
	ShareName       string `json:"shareName,omitempty"`
}
