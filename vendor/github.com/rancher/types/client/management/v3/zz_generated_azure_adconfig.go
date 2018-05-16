package client

const (
	AzureADConfigType                     = "azureADConfig"
	AzureADConfigFieldAccessMode          = "accessMode"
	AzureADConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	AzureADConfigFieldAnnotations         = "annotations"
	AzureADConfigFieldClientID            = "clientId"
	AzureADConfigFieldClientSecret        = "clientSecret"
	AzureADConfigFieldCreated             = "created"
	AzureADConfigFieldCreatorID           = "creatorId"
	AzureADConfigFieldDomain              = "domain"
	AzureADConfigFieldEnabled             = "enabled"
	AzureADConfigFieldLabels              = "labels"
	AzureADConfigFieldName                = "name"
	AzureADConfigFieldOwnerReferences     = "ownerReferences"
	AzureADConfigFieldRemoved             = "removed"
	AzureADConfigFieldTenantID            = "tenantId"
	AzureADConfigFieldType                = "type"
	AzureADConfigFieldUuid                = "uuid"
)

type AzureADConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClientID            string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret        string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Domain              string            `json:"domain,omitempty" yaml:"domain,omitempty"`
	Enabled             bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	TenantID            string            `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	Uuid                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
