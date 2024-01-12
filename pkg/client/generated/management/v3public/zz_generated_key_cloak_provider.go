package client

const (
	KeyCloakProviderType                  = "keyCloakProvider"
	KeyCloakProviderFieldAnnotations      = "annotations"
	KeyCloakProviderFieldAuthClientInfo   = "authClientInfo"
	KeyCloakProviderFieldCreated          = "created"
	KeyCloakProviderFieldCreatorID        = "creatorId"
	KeyCloakProviderFieldDeviceClientInfo = "deviceClientInfo"
	KeyCloakProviderFieldEndpoints        = "endpoints"
	KeyCloakProviderFieldLabels           = "labels"
	KeyCloakProviderFieldName             = "name"
	KeyCloakProviderFieldOwnerReferences  = "ownerReferences"
	KeyCloakProviderFieldRedirectURL      = "redirectUrl"
	KeyCloakProviderFieldRemoved          = "removed"
	KeyCloakProviderFieldScopes           = "scopes"
	KeyCloakProviderFieldType             = "type"
	KeyCloakProviderFieldUUID             = "uuid"
)

type KeyCloakProvider struct {
	Annotations      map[string]string       `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthClientInfo   *OAuthAuthorizationInfo `json:"authClientInfo,omitempty" yaml:"authClientInfo,omitempty"`
	Created          string                  `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID        string                  `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DeviceClientInfo *OAuthDeviceInfo        `json:"deviceClientInfo,omitempty" yaml:"deviceClientInfo,omitempty"`
	Endpoints        *OAuthEndpoint          `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Labels           map[string]string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name             string                  `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences  []OwnerReference        `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RedirectURL      string                  `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	Removed          string                  `json:"removed,omitempty" yaml:"removed,omitempty"`
	Scopes           []string                `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Type             string                  `json:"type,omitempty" yaml:"type,omitempty"`
	UUID             string                  `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
