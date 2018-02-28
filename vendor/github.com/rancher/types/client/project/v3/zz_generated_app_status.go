package client

const (
	AppStatusType            = "appStatus"
	AppStatusFieldConditions = "conditions"
	AppStatusFieldReleases   = "releases"
)

type AppStatus struct {
	Conditions []AppCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Releases   []ReleaseInfo  `json:"releases,omitempty" yaml:"releases,omitempty"`
}
