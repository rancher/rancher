package client

const (
	TokenType                 = "token"
	TokenFieldAnnotations     = "annotations"
	TokenFieldAuthProvider    = "authProvider"
	TokenFieldCreated         = "created"
	TokenFieldCreatorID       = "creatorId"
	TokenFieldDescription     = "description"
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
	TokenFieldUserID          = "userId"
	TokenFieldUserPrincipal   = "userPrincipal"
	TokenFieldUuid            = "uuid"
)

type Token struct {
	Annotations     map[string]string `json:"annotations,omitempty"`
	AuthProvider    string            `json:"authProvider,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Description     string            `json:"description,omitempty"`
	Expired         bool              `json:"expired,omitempty"`
	ExpiresAt       string            `json:"expiresAt,omitempty"`
	GroupPrincipals []string          `json:"groupPrincipals,omitempty"`
	IsDerived       bool              `json:"isDerived,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	LastUpdateTime  string            `json:"lastUpdateTime,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	TTLMillis       *int64            `json:"ttl,omitempty"`
	Token           string            `json:"token,omitempty"`
	UserID          string            `json:"userId,omitempty"`
	UserPrincipal   string            `json:"userPrincipal,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
