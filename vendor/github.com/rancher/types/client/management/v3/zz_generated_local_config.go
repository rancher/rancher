package client

const (
	LocalConfigType                 = "localConfig"
	LocalConfigFieldAnnotations     = "annotations"
	LocalConfigFieldCreated         = "created"
	LocalConfigFieldCreatorID       = "creatorId"
	LocalConfigFieldLabels          = "labels"
	LocalConfigFieldName            = "name"
	LocalConfigFieldOwnerReferences = "ownerReferences"
	LocalConfigFieldRemoved         = "removed"
	LocalConfigFieldUuid            = "uuid"
)

type LocalConfig struct {
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
