package client

const (
	CloudWatchConfigType                 = "cloudWatchConfig"
	CloudWatchConfigFieldAccessKeyID     = "accessKeyID"
	CloudWatchConfigFieldGroup           = "group"
	CloudWatchConfigFieldRegion          = "region"
	CloudWatchConfigFieldSecretAccessKey = "secretAccessKey"
	CloudWatchConfigFieldStream          = "stream"
)

type CloudWatchConfig struct {
	AccessKeyID     string `json:"accessKeyID,omitempty" yaml:"accessKeyID,omitempty"`
	Group           string `json:"group,omitempty" yaml:"group,omitempty"`
	Region          string `json:"region,omitempty" yaml:"region,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty" yaml:"secretAccessKey,omitempty"`
	Stream          string `json:"stream,omitempty" yaml:"stream,omitempty"`
}
