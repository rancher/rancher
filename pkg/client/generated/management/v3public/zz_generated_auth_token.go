package client

const (
	AuthTokenType                 = "authToken"
	AuthTokenFieldAnnotations     = "annotations"
	AuthTokenFieldCreated         = "created"
	AuthTokenFieldCreatorID       = "creatorId"
	AuthTokenFieldExpiresAt       = "expiresAt"
	AuthTokenFieldLabels          = "labels"
	AuthTokenFieldName            = "name"
	AuthTokenFieldOwnerReferences = "ownerReferences"
	AuthTokenFieldRemoved         = "removed"
	AuthTokenFieldToken           = "token"
	AuthTokenFieldUUID            = "uuid"
)

type AuthToken struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Token           string            `json:"token,omitempty" yaml:"token,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
