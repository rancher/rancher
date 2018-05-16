package client

const (
	AzureADProviderType                 = "azureADProvider"
	AzureADProviderFieldAnnotations     = "annotations"
	AzureADProviderFieldCreated         = "created"
	AzureADProviderFieldCreatorID       = "creatorId"
	AzureADProviderFieldLabels          = "labels"
	AzureADProviderFieldName            = "name"
	AzureADProviderFieldOwnerReferences = "ownerReferences"
	AzureADProviderFieldRemoved         = "removed"
	AzureADProviderFieldType            = "type"
	AzureADProviderFieldUuid            = "uuid"
)

type AzureADProvider struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	Uuid            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
