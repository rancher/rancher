package client

const (
	ThanosGCSSpecType           = "thanosGCSSpec"
	ThanosGCSSpecFieldBucket    = "bucket"
	ThanosGCSSpecFieldSecretKey = "credentials"
)

type ThanosGCSSpec struct {
	Bucket    string             `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	SecretKey *SecretKeySelector `json:"credentials,omitempty" yaml:"credentials,omitempty"`
}
