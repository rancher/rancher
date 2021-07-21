package client

const (
	ClusterUserAttributeType                 = "clusterUserAttribute"
	ClusterUserAttributeFieldAnnotations     = "annotations"
	ClusterUserAttributeFieldCreated         = "created"
	ClusterUserAttributeFieldCreatorID       = "creatorId"
	ClusterUserAttributeFieldEnabled         = "enabled"
	ClusterUserAttributeFieldExtraByProvider = "extraByProvider"
	ClusterUserAttributeFieldGroups          = "groups"
	ClusterUserAttributeFieldLabels          = "labels"
	ClusterUserAttributeFieldLastRefresh     = "lastRefresh"
	ClusterUserAttributeFieldName            = "name"
	ClusterUserAttributeFieldNamespaceId     = "namespaceId"
	ClusterUserAttributeFieldNeedsRefresh    = "needsRefresh"
	ClusterUserAttributeFieldOwnerReferences = "ownerReferences"
	ClusterUserAttributeFieldRemoved         = "removed"
	ClusterUserAttributeFieldUUID            = "uuid"
)

type ClusterUserAttribute struct {
	Annotations     map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled         bool                           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ExtraByProvider map[string]map[string][]string `json:"extraByProvider,omitempty" yaml:"extraByProvider,omitempty"`
	Groups          []string                       `json:"groups,omitempty" yaml:"groups,omitempty"`
	Labels          map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	LastRefresh     string                         `json:"lastRefresh,omitempty" yaml:"lastRefresh,omitempty"`
	Name            string                         `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId     string                         `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NeedsRefresh    bool                           `json:"needsRefresh,omitempty" yaml:"needsRefresh,omitempty"`
	OwnerReferences []OwnerReference               `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string                         `json:"removed,omitempty" yaml:"removed,omitempty"`
	UUID            string                         `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
