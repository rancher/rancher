package client

const (
	Sigv4Type           = "sigv4"
	Sigv4FieldAccessKey = "accessKey"
	Sigv4FieldProfile   = "profile"
	Sigv4FieldRegion    = "region"
	Sigv4FieldRoleArn   = "roleArn"
	Sigv4FieldSecretKey = "secretKey"
)

type Sigv4 struct {
	AccessKey *SecretKeySelector `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	Profile   string             `json:"profile,omitempty" yaml:"profile,omitempty"`
	Region    string             `json:"region,omitempty" yaml:"region,omitempty"`
	RoleArn   string             `json:"roleArn,omitempty" yaml:"roleArn,omitempty"`
	SecretKey *SecretKeySelector `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
