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
	ActiveDirectoryProviderFieldUuid               = "uuid"
)

type ActiveDirectoryProvider struct {
	Annotations        map[string]string `json:"annotations,omitempty"`
	Created            string            `json:"created,omitempty"`
	CreatorID          string            `json:"creatorId,omitempty"`
	DefaultLoginDomain string            `json:"defaultLoginDomain,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	Name               string            `json:"name,omitempty"`
	OwnerReferences    []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed            string            `json:"removed,omitempty"`
	Type               string            `json:"type,omitempty"`
	Uuid               string            `json:"uuid,omitempty"`
}
