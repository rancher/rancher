package client

const (
	PodCertificateProjectionType                      = "podCertificateProjection"
	PodCertificateProjectionFieldCertificateChainPath = "certificateChainPath"
	PodCertificateProjectionFieldCredentialBundlePath = "credentialBundlePath"
	PodCertificateProjectionFieldKeyPath              = "keyPath"
	PodCertificateProjectionFieldKeyType              = "keyType"
	PodCertificateProjectionFieldMaxExpirationSeconds = "maxExpirationSeconds"
	PodCertificateProjectionFieldSignerName           = "signerName"
)

type PodCertificateProjection struct {
	CertificateChainPath string `json:"certificateChainPath,omitempty" yaml:"certificateChainPath,omitempty"`
	CredentialBundlePath string `json:"credentialBundlePath,omitempty" yaml:"credentialBundlePath,omitempty"`
	KeyPath              string `json:"keyPath,omitempty" yaml:"keyPath,omitempty"`
	KeyType              string `json:"keyType,omitempty" yaml:"keyType,omitempty"`
	MaxExpirationSeconds *int64 `json:"maxExpirationSeconds,omitempty" yaml:"maxExpirationSeconds,omitempty"`
	SignerName           string `json:"signerName,omitempty" yaml:"signerName,omitempty"`
}
