package client

const (
	AppStatusType            = "appStatus"
	AppStatusFieldConditions = "conditions"
	AppStatusFieldReleases   = "releases"
)

type AppStatus struct {
	Conditions []AppCondition `json:"conditions,omitempty"`
	Releases   []ReleaseInfo  `json:"releases,omitempty"`
}
