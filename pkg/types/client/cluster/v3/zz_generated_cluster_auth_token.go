package client

const (
	ClusterAuthTokenType                 = "clusterAuthToken"
	ClusterAuthTokenFieldAnnotations     = "annotations"
	ClusterAuthTokenFieldCreated         = "created"
	ClusterAuthTokenFieldCreatorID       = "creatorId"
	ClusterAuthTokenFieldEnabled         = "enabled"
	ClusterAuthTokenFieldExpiresAt       = "expiresAt"
	ClusterAuthTokenFieldLabels          = "labels"
	ClusterAuthTokenFieldName            = "name"
	ClusterAuthTokenFieldNamespaceId     = "namespaceId"
	ClusterAuthTokenFieldOwnerReferences = "ownerReferences"
	ClusterAuthTokenFieldRemoved         = "removed"
	ClusterAuthTokenFieldSecretKeyHash   = "hash"
	ClusterAuthTokenFieldUUID            = "uuid"
	ClusterAuthTokenFieldUserName        = "userName"
)

type ClusterAuthToken struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled         bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SecretKeyHash   string            `json:"hash,omitempty" yaml:"hash,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserName        string            `json:"userName,omitempty" yaml:"userName,omitempty"`
}
