package client

const (
	AzureADProviderType                 = "azureADProvider"
	AzureADProviderFieldAnnotations     = "annotations"
	AzureADProviderFieldAuthURL         = "authUrl"
	AzureADProviderFieldClientID        = "clientId"
	AzureADProviderFieldCreated         = "created"
	AzureADProviderFieldCreatorID       = "creatorId"
	AzureADProviderFieldDeviceAuthURL   = "deviceAuthUrl"
	AzureADProviderFieldLabels          = "labels"
	AzureADProviderFieldName            = "name"
	AzureADProviderFieldOwnerReferences = "ownerReferences"
	AzureADProviderFieldRedirectURL     = "redirectUrl"
	AzureADProviderFieldRemoved         = "removed"
	AzureADProviderFieldScopes          = "scopes"
	AzureADProviderFieldTenantID        = "tenantId"
	AzureADProviderFieldTokenURL        = "tokenUrl"
	AzureADProviderFieldType            = "type"
	AzureADProviderFieldUUID            = "uuid"
)

type AzureADProvider struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthURL         string            `json:"authUrl,omitempty" yaml:"authUrl,omitempty"`
	ClientID        string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DeviceAuthURL   string            `json:"deviceAuthUrl,omitempty" yaml:"deviceAuthUrl,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RedirectURL     string            `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Scopes          []string          `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	TenantID        string            `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	TokenURL        string            `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
