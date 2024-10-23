package client

const (
	LocalProviderType                    = "localProvider"
	LocalProviderFieldAnnotations        = "annotations"
	LocalProviderFieldCreated            = "created"
	LocalProviderFieldCreatorID          = "creatorId"
	LocalProviderFieldLabels             = "labels"
	LocalProviderFieldLogoutAllEnabled   = "logoutAllEnabled"
	LocalProviderFieldLogoutAllForced    = "logoutAllForced"
	LocalProviderFieldLogoutAllSupported = "logoutAllSupported"
	LocalProviderFieldName               = "name"
	LocalProviderFieldOwnerReferences    = "ownerReferences"
	LocalProviderFieldRemoved            = "removed"
	LocalProviderFieldType               = "type"
	LocalProviderFieldUUID               = "uuid"
)

type LocalProvider struct {
	Annotations        map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created            string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID          string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels             map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LogoutAllEnabled   bool              `json:"logoutAllEnabled,omitempty" yaml:"logoutAllEnabled,omitempty"`
	LogoutAllForced    bool              `json:"logoutAllForced,omitempty" yaml:"logoutAllForced,omitempty"`
	LogoutAllSupported bool              `json:"logoutAllSupported,omitempty" yaml:"logoutAllSupported,omitempty"`
	Name               string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences    []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed            string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type               string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID               string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
