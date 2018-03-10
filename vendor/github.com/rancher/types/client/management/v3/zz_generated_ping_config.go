package client

const (
	PingConfigType                          = "pingConfig"
	PingConfigFieldAccessMode               = "accessMode"
	PingConfigFieldAllowedPrincipalIDs      = "allowedPrincipalIds"
	PingConfigFieldAnnotations              = "annotations"
	PingConfigFieldCreated                  = "created"
	PingConfigFieldCreatorID                = "creatorId"
	PingConfigFieldDisplayNameField         = "displayNameField"
	PingConfigFieldEnabled                  = "enabled"
	PingConfigFieldGroupsField              = "groupsField"
	PingConfigFieldIDPMetadataContent       = "idpMetadataContent"
	PingConfigFieldIDPMetadataFilePath      = "idpMetadataFilePath"
	PingConfigFieldIDPMetadataURL           = "idpMetadataUrl"
	PingConfigFieldLabels                   = "labels"
	PingConfigFieldName                     = "name"
	PingConfigFieldOwnerReferences          = "ownerReferences"
	PingConfigFieldRancherAPIHost           = "rancherApiHost"
	PingConfigFieldRemoved                  = "removed"
	PingConfigFieldSPSelfSignedCert         = "spCert"
	PingConfigFieldSPSelfSignedCertFilePath = "spSelfSignedCertFilePath"
	PingConfigFieldSPSelfSignedKey          = "spKey"
	PingConfigFieldSPSelfSignedKeyFilePath  = "spSelfSignedKeyFilePath"
	PingConfigFieldType                     = "type"
	PingConfigFieldUIDField                 = "uidField"
	PingConfigFieldUserNameField            = "userNameField"
	PingConfigFieldUuid                     = "uuid"
)

type PingConfig struct {
	AccessMode               string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs      []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations              map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                  string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DisplayNameField         string            `json:"displayNameField,omitempty" yaml:"displayNameField,omitempty"`
	Enabled                  bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupsField              string            `json:"groupsField,omitempty" yaml:"groupsField,omitempty"`
	IDPMetadataContent       string            `json:"idpMetadataContent,omitempty" yaml:"idpMetadataContent,omitempty"`
	IDPMetadataFilePath      string            `json:"idpMetadataFilePath,omitempty" yaml:"idpMetadataFilePath,omitempty"`
	IDPMetadataURL           string            `json:"idpMetadataUrl,omitempty" yaml:"idpMetadataUrl,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                     string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences          []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RancherAPIHost           string            `json:"rancherApiHost,omitempty" yaml:"rancherApiHost,omitempty"`
	Removed                  string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SPSelfSignedCert         string            `json:"spCert,omitempty" yaml:"spCert,omitempty"`
	SPSelfSignedCertFilePath string            `json:"spSelfSignedCertFilePath,omitempty" yaml:"spSelfSignedCertFilePath,omitempty"`
	SPSelfSignedKey          string            `json:"spKey,omitempty" yaml:"spKey,omitempty"`
	SPSelfSignedKeyFilePath  string            `json:"spSelfSignedKeyFilePath,omitempty" yaml:"spSelfSignedKeyFilePath,omitempty"`
	Type                     string            `json:"type,omitempty" yaml:"type,omitempty"`
	UIDField                 string            `json:"uidField,omitempty" yaml:"uidField,omitempty"`
	UserNameField            string            `json:"userNameField,omitempty" yaml:"userNameField,omitempty"`
	Uuid                     string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
