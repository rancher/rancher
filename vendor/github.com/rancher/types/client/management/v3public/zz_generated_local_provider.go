package client

const (
	LocalProviderType                 = "localProvider"
	LocalProviderFieldAnnotations     = "annotations"
	LocalProviderFieldCreated         = "created"
	LocalProviderFieldCreatorID       = "creatorId"
	LocalProviderFieldLabels          = "labels"
	LocalProviderFieldName            = "name"
	LocalProviderFieldOwnerReferences = "ownerReferences"
	LocalProviderFieldRemoved         = "removed"
	LocalProviderFieldType            = "type"
	LocalProviderFieldUuid            = "uuid"
)

type LocalProvider struct {
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Type            string            `json:"type,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
