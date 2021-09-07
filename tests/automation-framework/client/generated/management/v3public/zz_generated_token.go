package client

const (
	TokenType                 = "token"
	TokenFieldAnnotations     = "annotations"
	TokenFieldAuthProvider    = "authProvider"
	TokenFieldClusterID       = "clusterId"
	TokenFieldCreated         = "created"
	TokenFieldCreatorID       = "creatorId"
	TokenFieldCurrent         = "current"
	TokenFieldDescription     = "description"
	TokenFieldEnabled         = "enabled"
	TokenFieldExpired         = "expired"
	TokenFieldExpiresAt       = "expiresAt"
	TokenFieldGroupPrincipals = "groupPrincipals"
	TokenFieldIsDerived       = "isDerived"
	TokenFieldLabels          = "labels"
	TokenFieldLastUpdateTime  = "lastUpdateTime"
	TokenFieldName            = "name"
	TokenFieldOwnerReferences = "ownerReferences"
	TokenFieldProviderInfo    = "providerInfo"
	TokenFieldRemoved         = "removed"
	TokenFieldTTLMillis       = "ttl"
	TokenFieldToken           = "token"
	TokenFieldUUID            = "uuid"
	TokenFieldUserID          = "userId"
	TokenFieldUserPrincipal   = "userPrincipal"
)

type Token struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthProvider    string            `json:"authProvider,omitempty" yaml:"authProvider,omitempty"`
	ClusterID       string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Current         bool              `json:"current,omitempty" yaml:"current,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled         *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Expired         bool              `json:"expired,omitempty" yaml:"expired,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	GroupPrincipals []string          `json:"groupPrincipals,omitempty" yaml:"groupPrincipals,omitempty"`
	IsDerived       bool              `json:"isDerived,omitempty" yaml:"isDerived,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastUpdateTime  string            `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty" yaml:"providerInfo,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	TTLMillis       int64             `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Token           string            `json:"token,omitempty" yaml:"token,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserID          string            `json:"userId,omitempty" yaml:"userId,omitempty"`
	UserPrincipal   string            `json:"userPrincipal,omitempty" yaml:"userPrincipal,omitempty"`
}
