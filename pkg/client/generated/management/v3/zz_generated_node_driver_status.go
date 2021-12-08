package client

const (
	NodeDriverStatusType                             = "nodeDriverStatus"
	NodeDriverStatusFieldAppliedChecksum             = "appliedChecksum"
	NodeDriverStatusFieldAppliedDockerMachineVersion = "appliedDockerMachineVersion"
	NodeDriverStatusFieldAppliedURL                  = "appliedURL"
	NodeDriverStatusFieldConditions                  = "conditions"
)

type NodeDriverStatus struct {
	AppliedChecksum             string      `json:"appliedChecksum,omitempty" yaml:"appliedChecksum,omitempty"`
	AppliedDockerMachineVersion string      `json:"appliedDockerMachineVersion,omitempty" yaml:"appliedDockerMachineVersion,omitempty"`
	AppliedURL                  string      `json:"appliedURL,omitempty" yaml:"appliedURL,omitempty"`
	Conditions                  []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
