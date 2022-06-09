package client

const (
	KeyCloakOIDCProviderType                 = "keyCloakOIDCProvider"
	KeyCloakOIDCProviderFieldAnnotations     = "annotations"
	KeyCloakOIDCProviderFieldCreated         = "created"
	KeyCloakOIDCProviderFieldCreatorID       = "creatorId"
	KeyCloakOIDCProviderFieldLabels          = "labels"
	KeyCloakOIDCProviderFieldName            = "name"
	KeyCloakOIDCProviderFieldOwnerReferences = "ownerReferences"
	KeyCloakOIDCProviderFieldRedirectURL     = "redirectUrl"
	KeyCloakOIDCProviderFieldRemoved         = "removed"
	KeyCloakOIDCProviderFieldType            = "type"
	KeyCloakOIDCProviderFieldUUID            = "uuid"
)

type KeyCloakOIDCProvider struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RedirectURL     string            `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
