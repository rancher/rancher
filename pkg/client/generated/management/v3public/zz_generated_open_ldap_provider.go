package client

const (
	OpenLdapProviderType                  = "openLdapProvider"
	OpenLdapProviderFieldAnnotations      = "annotations"
	OpenLdapProviderFieldAuthClientInfo   = "authClientInfo"
	OpenLdapProviderFieldCreated          = "created"
	OpenLdapProviderFieldCreatorID        = "creatorId"
	OpenLdapProviderFieldDeviceClientInfo = "deviceClientInfo"
	OpenLdapProviderFieldEndpoints        = "endpoints"
	OpenLdapProviderFieldLabels           = "labels"
	OpenLdapProviderFieldName             = "name"
	OpenLdapProviderFieldOwnerReferences  = "ownerReferences"
	OpenLdapProviderFieldRemoved          = "removed"
	OpenLdapProviderFieldScopes           = "scopes"
	OpenLdapProviderFieldType             = "type"
	OpenLdapProviderFieldUUID             = "uuid"
)

type OpenLdapProvider struct {
	Annotations      map[string]string       `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthClientInfo   *OAuthAuthorizationInfo `json:"authClientInfo,omitempty" yaml:"authClientInfo,omitempty"`
	Created          string                  `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID        string                  `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DeviceClientInfo *OAuthDeviceInfo        `json:"deviceClientInfo,omitempty" yaml:"deviceClientInfo,omitempty"`
	Endpoints        *OAuthEndpoint          `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Labels           map[string]string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name             string                  `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences  []OwnerReference        `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed          string                  `json:"removed,omitempty" yaml:"removed,omitempty"`
	Scopes           []string                `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Type             string                  `json:"type,omitempty" yaml:"type,omitempty"`
	UUID             string                  `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
