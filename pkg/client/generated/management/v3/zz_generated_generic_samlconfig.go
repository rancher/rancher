package client

const (
	GenericSAMLConfigType                     = "genericSAMLConfig"
	GenericSAMLConfigFieldAccessMode          = "accessMode"
	GenericSAMLConfigFieldAllowIdpInitiated   = "allowIdpInitiated"
	GenericSAMLConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	GenericSAMLConfigFieldAnnotations         = "annotations"
	GenericSAMLConfigFieldCreated             = "created"
	GenericSAMLConfigFieldCreatorID           = "creatorId"
	GenericSAMLConfigFieldDisplayNameField    = "displayNameField"
	GenericSAMLConfigFieldEnabled             = "enabled"
	GenericSAMLConfigFieldEntityID            = "entityID"
	GenericSAMLConfigFieldForceAuthn          = "forceAuthn"
	GenericSAMLConfigFieldGroupsField         = "groupsField"
	GenericSAMLConfigFieldIDPMetadataContent  = "idpMetadataContent"
	GenericSAMLConfigFieldLabels              = "labels"
	GenericSAMLConfigFieldLogoutAllEnabled    = "logoutAllEnabled"
	GenericSAMLConfigFieldLogoutAllForced     = "logoutAllForced"
	GenericSAMLConfigFieldLogoutAllSupported  = "logoutAllSupported"
	GenericSAMLConfigFieldName                = "name"
	GenericSAMLConfigFieldNameIDFormat        = "nameIDFormat"
	GenericSAMLConfigFieldOwnerReferences     = "ownerReferences"
	GenericSAMLConfigFieldRancherAPIHost      = "rancherApiHost"
	GenericSAMLConfigFieldRemoved             = "removed"
	GenericSAMLConfigFieldSignatureMethod     = "signatureMethod"
	GenericSAMLConfigFieldSpCert              = "spCert"
	GenericSAMLConfigFieldSpKey               = "spKey"
	GenericSAMLConfigFieldStatus              = "status"
	GenericSAMLConfigFieldType                = "type"
	GenericSAMLConfigFieldUIDField            = "uidField"
	GenericSAMLConfigFieldUUID                = "uuid"
	GenericSAMLConfigFieldUserNameField       = "userNameField"
)

type GenericSAMLConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowIdpInitiated   bool              `json:"allowIdpInitiated,omitempty" yaml:"allowIdpInitiated,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DisplayNameField    string            `json:"displayNameField,omitempty" yaml:"displayNameField,omitempty"`
	Enabled             bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	EntityID            string            `json:"entityID,omitempty" yaml:"entityID,omitempty"`
	ForceAuthn          *bool             `json:"forceAuthn,omitempty" yaml:"forceAuthn,omitempty"`
	GroupsField         string            `json:"groupsField,omitempty" yaml:"groupsField,omitempty"`
	IDPMetadataContent  string            `json:"idpMetadataContent,omitempty" yaml:"idpMetadataContent,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LogoutAllEnabled    bool              `json:"logoutAllEnabled,omitempty" yaml:"logoutAllEnabled,omitempty"`
	LogoutAllForced     bool              `json:"logoutAllForced,omitempty" yaml:"logoutAllForced,omitempty"`
	LogoutAllSupported  bool              `json:"logoutAllSupported,omitempty" yaml:"logoutAllSupported,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	NameIDFormat        string            `json:"nameIDFormat,omitempty" yaml:"nameIDFormat,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RancherAPIHost      string            `json:"rancherApiHost,omitempty" yaml:"rancherApiHost,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SignatureMethod     string            `json:"signatureMethod,omitempty" yaml:"signatureMethod,omitempty"`
	SpCert              string            `json:"spCert,omitempty" yaml:"spCert,omitempty"`
	SpKey               string            `json:"spKey,omitempty" yaml:"spKey,omitempty"`
	Status              *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	UIDField            string            `json:"uidField,omitempty" yaml:"uidField,omitempty"`
	UUID                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UserNameField       string            `json:"userNameField,omitempty" yaml:"userNameField,omitempty"`
}
