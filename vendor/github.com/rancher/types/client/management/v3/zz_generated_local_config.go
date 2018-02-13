package client

const (
	LocalConfigType                     = "localConfig"
	LocalConfigFieldAccessMode          = "accessMode"
	LocalConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	LocalConfigFieldAnnotations         = "annotations"
	LocalConfigFieldCreated             = "created"
	LocalConfigFieldCreatorID           = "creatorId"
	LocalConfigFieldEnabled             = "enabled"
	LocalConfigFieldLabels              = "labels"
	LocalConfigFieldName                = "name"
	LocalConfigFieldOwnerReferences     = "ownerReferences"
	LocalConfigFieldRemoved             = "removed"
	LocalConfigFieldType                = "type"
	LocalConfigFieldUuid                = "uuid"
)

type LocalConfig struct {
	AccessMode          string            `json:"accessMode,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty"`
	Created             string            `json:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty"`
	Enabled             bool              `json:"enabled,omitempty"`
	Labels              map[string]string `json:"labels,omitempty"`
	Name                string            `json:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed             string            `json:"removed,omitempty"`
	Type                string            `json:"type,omitempty"`
	Uuid                string            `json:"uuid,omitempty"`
}
