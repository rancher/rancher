package client

const (
	AlibabaCredentialConfigType                 = "alibabaCredentialConfig"
	AlibabaCredentialConfigFieldAccessKeyId     = "accessKeyId"
	AlibabaCredentialConfigFieldAccessKeySecret = "accessKeySecret"
)

type AlibabaCredentialConfig struct {
	AccessKeyId     string `json:"accessKeyId,omitempty" yaml:"accessKeyId,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty" yaml:"accessKeySecret,omitempty"`
}
