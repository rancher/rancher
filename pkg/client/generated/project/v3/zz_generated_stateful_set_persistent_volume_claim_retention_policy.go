package client

const (
	StatefulSetPersistentVolumeClaimRetentionPolicyType             = "statefulSetPersistentVolumeClaimRetentionPolicy"
	StatefulSetPersistentVolumeClaimRetentionPolicyFieldWhenDeleted = "whenDeleted"
	StatefulSetPersistentVolumeClaimRetentionPolicyFieldWhenScaled  = "whenScaled"
)

type StatefulSetPersistentVolumeClaimRetentionPolicy struct {
	WhenDeleted string `json:"whenDeleted,omitempty" yaml:"whenDeleted,omitempty"`
	WhenScaled  string `json:"whenScaled,omitempty" yaml:"whenScaled,omitempty"`
}
