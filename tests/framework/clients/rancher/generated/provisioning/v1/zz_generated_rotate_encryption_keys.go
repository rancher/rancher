package client

const (
	RotateEncryptionKeysType            = "rotateEncryptionKeys"
	RotateEncryptionKeysFieldGeneration = "generation"
)

type RotateEncryptionKeys struct {
	Generation int64 `json:"generation,omitempty" yaml:"generation,omitempty"`
}
