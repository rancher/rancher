package client

const (
	FleetWorkspacePermissionType                = "fleetWorkspacePermission"
	FleetWorkspacePermissionFieldResourceRules  = "resourceRules"
	FleetWorkspacePermissionFieldWorkspaceVerbs = "workspaceVerbs"
)

type FleetWorkspacePermission struct {
	ResourceRules  []PolicyRule `json:"resourceRules,omitempty" yaml:"resourceRules,omitempty"`
	WorkspaceVerbs []string     `json:"workspaceVerbs,omitempty" yaml:"workspaceVerbs,omitempty"`
}
