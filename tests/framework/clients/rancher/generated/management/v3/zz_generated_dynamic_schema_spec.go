package client

const (
	DynamicSchemaSpecType                      = "dynamicSchemaSpec"
	DynamicSchemaSpecFieldCollectionActions    = "collectionActions"
	DynamicSchemaSpecFieldCollectionFields     = "collectionFields"
	DynamicSchemaSpecFieldCollectionFilters    = "collectionFilters"
	DynamicSchemaSpecFieldCollectionMethods    = "collectionMethods"
	DynamicSchemaSpecFieldDynamicSchemaVersion = "dynamicSchemaVersion"
	DynamicSchemaSpecFieldEmbed                = "embed"
	DynamicSchemaSpecFieldEmbedType            = "embedType"
	DynamicSchemaSpecFieldIncludeableLinks     = "includeableLinks"
	DynamicSchemaSpecFieldPluralName           = "pluralName"
	DynamicSchemaSpecFieldResourceActions      = "resourceActions"
	DynamicSchemaSpecFieldResourceFields       = "resourceFields"
	DynamicSchemaSpecFieldResourceMethods      = "resourceMethods"
	DynamicSchemaSpecFieldSchemaName           = "schemaName"
)

type DynamicSchemaSpec struct {
	CollectionActions    map[string]Action `json:"collectionActions,omitempty" yaml:"collectionActions,omitempty"`
	CollectionFields     map[string]Field  `json:"collectionFields,omitempty" yaml:"collectionFields,omitempty"`
	CollectionFilters    map[string]Filter `json:"collectionFilters,omitempty" yaml:"collectionFilters,omitempty"`
	CollectionMethods    []string          `json:"collectionMethods,omitempty" yaml:"collectionMethods,omitempty"`
	DynamicSchemaVersion string            `json:"dynamicSchemaVersion,omitempty" yaml:"dynamicSchemaVersion,omitempty"`
	Embed                bool              `json:"embed,omitempty" yaml:"embed,omitempty"`
	EmbedType            string            `json:"embedType,omitempty" yaml:"embedType,omitempty"`
	IncludeableLinks     []string          `json:"includeableLinks,omitempty" yaml:"includeableLinks,omitempty"`
	PluralName           string            `json:"pluralName,omitempty" yaml:"pluralName,omitempty"`
	ResourceActions      map[string]Action `json:"resourceActions,omitempty" yaml:"resourceActions,omitempty"`
	ResourceFields       map[string]Field  `json:"resourceFields,omitempty" yaml:"resourceFields,omitempty"`
	ResourceMethods      []string          `json:"resourceMethods,omitempty" yaml:"resourceMethods,omitempty"`
	SchemaName           string            `json:"schemaName,omitempty" yaml:"schemaName,omitempty"`
}
