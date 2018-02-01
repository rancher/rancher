package client

const (
	AppStatusType          = "appStatus"
	AppStatusFieldReleases = "releases"
)

type AppStatus struct {
	Releases []ReleaseInfo `json:"releases,omitempty"`
}
