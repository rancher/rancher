package client

const (
	OpenLDAPProviderType                    = "openLDAPProvider"
	OpenLDAPProviderFieldAnnotations        = "annotations"
	OpenLDAPProviderFieldCreated            = "created"
	OpenLDAPProviderFieldCreatorID          = "creatorId"
	OpenLDAPProviderFieldDefaultLoginDomain = "defaultLoginDomain"
	OpenLDAPProviderFieldLabels             = "labels"
	OpenLDAPProviderFieldName               = "name"
	OpenLDAPProviderFieldOwnerReferences    = "ownerReferences"
	OpenLDAPProviderFieldRemoved            = "removed"
	OpenLDAPProviderFieldType               = "type"
	OpenLDAPProviderFieldUuid               = "uuid"
)

type OpenLDAPProvider struct {
	Annotations        map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created            string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID          string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultLoginDomain string            `json:"defaultLoginDomain,omitempty" yaml:"defaultLoginDomain,omitempty"`
	Labels             map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name               string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences    []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed            string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type               string            `json:"type,omitempty" yaml:"type,omitempty"`
	Uuid               string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
