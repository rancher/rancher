package client

const (
	PersistentVolumeClaimTemplateType            = "persistentVolumeClaimTemplate"
	PersistentVolumeClaimTemplateFieldObjectMeta = "metadata"
	PersistentVolumeClaimTemplateFieldSpec       = "spec"
)

type PersistentVolumeClaimTemplate struct {
	ObjectMeta *ObjectMeta                `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       *PersistentVolumeClaimSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}
