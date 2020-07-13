package client

const (
	FieldType              = "field"
	FieldFieldCreate       = "create"
	FieldFieldDefault      = "default"
	FieldFieldDescription  = "description"
	FieldFieldDynamicField = "dynamicField"
	FieldFieldInvalidChars = "invalidChars"
	FieldFieldMax          = "max"
	FieldFieldMaxLength    = "maxLength"
	FieldFieldMin          = "min"
	FieldFieldMinLength    = "minLength"
	FieldFieldNullable     = "nullable"
	FieldFieldOptions      = "options"
	FieldFieldRequired     = "required"
	FieldFieldType         = "type"
	FieldFieldUnique       = "unique"
	FieldFieldUpdate       = "update"
	FieldFieldValidChars   = "validChars"
)

type Field struct {
	Create       bool     `json:"create,omitempty" yaml:"create,omitempty"`
	Default      *Values  `json:"default,omitempty" yaml:"default,omitempty"`
	Description  string   `json:"description,omitempty" yaml:"description,omitempty"`
	DynamicField bool     `json:"dynamicField,omitempty" yaml:"dynamicField,omitempty"`
	InvalidChars string   `json:"invalidChars,omitempty" yaml:"invalidChars,omitempty"`
	Max          int64    `json:"max,omitempty" yaml:"max,omitempty"`
	MaxLength    int64    `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Min          int64    `json:"min,omitempty" yaml:"min,omitempty"`
	MinLength    int64    `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	Nullable     bool     `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	Options      []string `json:"options,omitempty" yaml:"options,omitempty"`
	Required     bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Type         string   `json:"type,omitempty" yaml:"type,omitempty"`
	Unique       bool     `json:"unique,omitempty" yaml:"unique,omitempty"`
	Update       bool     `json:"update,omitempty" yaml:"update,omitempty"`
	ValidChars   string   `json:"validChars,omitempty" yaml:"validChars,omitempty"`
}
