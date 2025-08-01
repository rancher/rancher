package client

const (
	AlibabaCredentialConfigType                 = "alibabaCredentialConfig"
	AlibabaCredentialConfigFieldAccessKeyID     = "accessKeyId"
	AlibabaCredentialConfigFieldAccessKeySecret = "accessKeySecret"
)

type AlibabaCredentialConfig struct {
	AccessKeyID     string `json:"accessKeyId,omitempty" yaml:"accessKeyId,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty" yaml:"accessKeySecret,omitempty"`
}
