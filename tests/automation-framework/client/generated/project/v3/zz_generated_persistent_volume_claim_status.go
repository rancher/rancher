package client

const (
	PersistentVolumeClaimStatusType             = "persistentVolumeClaimStatus"
	PersistentVolumeClaimStatusFieldAccessModes = "accessModes"
	PersistentVolumeClaimStatusFieldCapacity    = "capacity"
	PersistentVolumeClaimStatusFieldConditions  = "conditions"
	PersistentVolumeClaimStatusFieldPhase       = "phase"
)

type PersistentVolumeClaimStatus struct {
	AccessModes []string                         `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	Capacity    map[string]string                `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	Conditions  []PersistentVolumeClaimCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Phase       string                           `json:"phase,omitempty" yaml:"phase,omitempty"`
}
