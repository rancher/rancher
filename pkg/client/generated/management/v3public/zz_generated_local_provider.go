package client

const (
	LocalProviderType                  = "localProvider"
	LocalProviderFieldAnnotations      = "annotations"
	LocalProviderFieldAuthClientInfo   = "authClientInfo"
	LocalProviderFieldCreated          = "created"
	LocalProviderFieldCreatorID        = "creatorId"
	LocalProviderFieldDeviceClientInfo = "deviceClientInfo"
	LocalProviderFieldEndpoints        = "endpoints"
	LocalProviderFieldLabels           = "labels"
	LocalProviderFieldName             = "name"
	LocalProviderFieldOwnerReferences  = "ownerReferences"
	LocalProviderFieldRemoved          = "removed"
	LocalProviderFieldScopes           = "scopes"
	LocalProviderFieldType             = "type"
	LocalProviderFieldUUID             = "uuid"
)

type LocalProvider struct {
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
