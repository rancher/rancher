package client

const (
	EncryptionConfigurationType            = "encryptionConfiguration"
	EncryptionConfigurationFieldAPIVersion = "apiVersion"
	EncryptionConfigurationFieldKind       = "kind"
	EncryptionConfigurationFieldResources  = "resources"
)

type EncryptionConfiguration struct {
	APIVersion string                  `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string                  `json:"kind,omitempty" yaml:"kind,omitempty"`
	Resources  []ResourceConfiguration `json:"resources,omitempty" yaml:"resources,omitempty"`
}
