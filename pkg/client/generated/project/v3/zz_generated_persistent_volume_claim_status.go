package client

const (
	PersistentVolumeClaimStatusType                    = "persistentVolumeClaimStatus"
	PersistentVolumeClaimStatusFieldAccessModes        = "accessModes"
	PersistentVolumeClaimStatusFieldAllocatedResources = "allocatedResources"
	PersistentVolumeClaimStatusFieldCapacity           = "capacity"
	PersistentVolumeClaimStatusFieldConditions         = "conditions"
	PersistentVolumeClaimStatusFieldPhase              = "phase"
	PersistentVolumeClaimStatusFieldResizeStatus       = "resizeStatus"
)

type PersistentVolumeClaimStatus struct {
	AccessModes        []string                         `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	AllocatedResources map[string]string                `json:"allocatedResources,omitempty" yaml:"allocatedResources,omitempty"`
	Capacity           map[string]string                `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	Conditions         []PersistentVolumeClaimCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Phase              string                           `json:"phase,omitempty" yaml:"phase,omitempty"`
	ResizeStatus       string                           `json:"resizeStatus,omitempty" yaml:"resizeStatus,omitempty"`
}
