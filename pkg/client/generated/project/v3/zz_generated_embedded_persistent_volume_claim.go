package client

const (
	EmbeddedPersistentVolumeClaimType                        = "embeddedPersistentVolumeClaim"
	EmbeddedPersistentVolumeClaimFieldAPIVersion             = "apiVersion"
	EmbeddedPersistentVolumeClaimFieldEmbeddedObjectMetadata = "metadata"
	EmbeddedPersistentVolumeClaimFieldKind                   = "kind"
	EmbeddedPersistentVolumeClaimFieldSpec                   = "spec"
	EmbeddedPersistentVolumeClaimFieldStatus                 = "status"
)

type EmbeddedPersistentVolumeClaim struct {
	APIVersion             string                       `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	EmbeddedObjectMetadata *EmbeddedObjectMetadata      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Kind                   string                       `json:"kind,omitempty" yaml:"kind,omitempty"`
	Spec                   *PersistentVolumeClaimSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status                 *PersistentVolumeClaimStatus `json:"status,omitempty" yaml:"status,omitempty"`
}
