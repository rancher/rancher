package client

const (
	UpdateGlobalDNSTargetsInputType            = "updateGlobalDNSTargetsInput"
	UpdateGlobalDNSTargetsInputFieldProjectIDs = "projectIds"
)

type UpdateGlobalDNSTargetsInput struct {
	ProjectIDs []string `json:"projectIds,omitempty" yaml:"projectIds,omitempty"`
}
