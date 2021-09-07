package client

const (
	ActiveDirectoryProviderType                    = "activeDirectoryProvider"
	ActiveDirectoryProviderFieldAnnotations        = "annotations"
	ActiveDirectoryProviderFieldCreated            = "created"
	ActiveDirectoryProviderFieldCreatorID          = "creatorId"
	ActiveDirectoryProviderFieldDefaultLoginDomain = "defaultLoginDomain"
	ActiveDirectoryProviderFieldLabels             = "labels"
	ActiveDirectoryProviderFieldName               = "name"
	ActiveDirectoryProviderFieldOwnerReferences    = "ownerReferences"
	ActiveDirectoryProviderFieldRemoved            = "removed"
	ActiveDirectoryProviderFieldType               = "type"
	ActiveDirectoryProviderFieldUUID               = "uuid"
)

type ActiveDirectoryProvider struct {
	Annotations        map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created            string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID          string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultLoginDomain string            `json:"defaultLoginDomain,omitempty" yaml:"defaultLoginDomain,omitempty"`
	Labels             map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name               string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences    []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed            string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type               string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID               string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
