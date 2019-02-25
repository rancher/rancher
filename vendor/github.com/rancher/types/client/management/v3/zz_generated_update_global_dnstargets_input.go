package client

const (
	UpdateGlobalDNSTargetsInputType                   = "updateGlobalDNSTargetsInput"
	UpdateGlobalDNSTargetsInputFieldMultiClusterAppID = "multiClusterAppId"
	UpdateGlobalDNSTargetsInputFieldProjectIDs        = "projectIds"
)

type UpdateGlobalDNSTargetsInput struct {
	MultiClusterAppID string   `json:"multiClusterAppId,omitempty" yaml:"multiClusterAppId,omitempty"`
	ProjectIDs        []string `json:"projectIds,omitempty" yaml:"projectIds,omitempty"`
}
