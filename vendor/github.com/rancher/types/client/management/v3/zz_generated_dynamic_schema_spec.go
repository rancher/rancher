package client

const (
	DynamicSchemaSpecType                   = "dynamicSchemaSpec"
	DynamicSchemaSpecFieldCollectionActions = "collectionActions"
	DynamicSchemaSpecFieldCollectionFields  = "collectionFields"
	DynamicSchemaSpecFieldCollectionFilters = "collectionFilters"
	DynamicSchemaSpecFieldCollectionMethods = "collectionMethods"
	DynamicSchemaSpecFieldEmbed             = "embed"
	DynamicSchemaSpecFieldEmbedType         = "embedType"
	DynamicSchemaSpecFieldIncludeableLinks  = "includeableLinks"
	DynamicSchemaSpecFieldPluralName        = "pluralName"
	DynamicSchemaSpecFieldResourceActions   = "resourceActions"
	DynamicSchemaSpecFieldResourceFields    = "resourceFields"
	DynamicSchemaSpecFieldResourceMethods   = "resourceMethods"
)

type DynamicSchemaSpec struct {
	CollectionActions map[string]Action `json:"collectionActions,omitempty"`
	CollectionFields  map[string]Field  `json:"collectionFields,omitempty"`
	CollectionFilters map[string]Filter `json:"collectionFilters,omitempty"`
	CollectionMethods []string          `json:"collectionMethods,omitempty"`
	Embed             bool              `json:"embed,omitempty"`
	EmbedType         string            `json:"embedType,omitempty"`
	IncludeableLinks  []string          `json:"includeableLinks,omitempty"`
	PluralName        string            `json:"pluralName,omitempty"`
	ResourceActions   map[string]Action `json:"resourceActions,omitempty"`
	ResourceFields    map[string]Field  `json:"resourceFields,omitempty"`
	ResourceMethods   []string          `json:"resourceMethods,omitempty"`
}
