package client

const (
	UserAttributeType                 = "userAttribute"
	UserAttributeFieldAnnotations     = "annotations"
	UserAttributeFieldCreated         = "created"
	UserAttributeFieldCreatorID       = "creatorId"
	UserAttributeFieldGroupPrincipals = "groupPrincipals"
	UserAttributeFieldLabels          = "labels"
	UserAttributeFieldLastRefresh     = "lastRefresh"
	UserAttributeFieldName            = "name"
	UserAttributeFieldNeedsRefresh    = "needsRefresh"
	UserAttributeFieldOwnerReferences = "ownerReferences"
	UserAttributeFieldRemoved         = "removed"
	UserAttributeFieldUUID            = "uuid"
	UserAttributeFieldUserName        = "userName"
)

type UserAttribute struct {
	Annotations     map[string]string    `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string               `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string               `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	GroupPrincipals map[string]Principal `json:"groupPrincipals,omitempty" yaml:"groupPrincipals,omitempty"`
	Labels          map[string]string    `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastRefresh     string               `json:"lastRefresh,omitempty" yaml:"lastRefresh,omitempty"`
	Name            string               `json:"name,omitempty" yaml:"name,omitempty"`
	NeedsRefresh    bool                 `json:"needsRefresh,omitempty" yaml:"needsRefresh,omitempty"`
	OwnerReferences []OwnerReference     `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string               `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string               `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserName        string               `json:"userName,omitempty" yaml:"userName,omitempty"`
}
