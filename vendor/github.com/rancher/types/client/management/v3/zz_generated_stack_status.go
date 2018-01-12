package client

const (
	StackStatusType          = "stackStatus"
	StackStatusFieldReleases = "releases"
)

type StackStatus struct {
	Releases []ReleaseInfo `json:"releases,omitempty"`
}
