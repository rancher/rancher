package client

const (
	AppStatusType            = "appStatus"
	AppStatusFieldConditions = "conditions"
	AppStatusFieldReleases   = "releases"
	AppStatusFieldStdError   = "stdError"
	AppStatusFieldStdOutput  = "stdOutput"
)

type AppStatus struct {
	Conditions []AppCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Releases   []ReleaseInfo  `json:"releases,omitempty" yaml:"releases,omitempty"`
	StdError   []string       `json:"stdError,omitempty" yaml:"stdError,omitempty"`
	StdOutput  []string       `json:"stdOutput,omitempty" yaml:"stdOutput,omitempty"`
}
