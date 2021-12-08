package client

const (
	TargetType             = "target"
	TargetFieldAppID       = "appId"
	TargetFieldHealthstate = "healthState"
	TargetFieldProjectID   = "projectId"
	TargetFieldState       = "state"
)

type Target struct {
	AppID       string `json:"appId,omitempty" yaml:"appId,omitempty"`
	Healthstate string `json:"healthState,omitempty" yaml:"healthState,omitempty"`
	ProjectID   string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	State       string `json:"state,omitempty" yaml:"state,omitempty"`
}
