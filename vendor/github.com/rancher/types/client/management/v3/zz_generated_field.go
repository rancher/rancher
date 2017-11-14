package client

const (
	FieldType              = "field"
	FieldFieldCreate       = "create"
	FieldFieldDefault      = "default"
	FieldFieldDescription  = "description"
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
	Create       *bool    `json:"create,omitempty"`
	Default      *Values  `json:"default,omitempty"`
	Description  string   `json:"description,omitempty"`
	InvalidChars string   `json:"invalidChars,omitempty"`
	Max          *int64   `json:"max,omitempty"`
	MaxLength    *int64   `json:"maxLength,omitempty"`
	Min          *int64   `json:"min,omitempty"`
	MinLength    *int64   `json:"minLength,omitempty"`
	Nullable     *bool    `json:"nullable,omitempty"`
	Options      []string `json:"options,omitempty"`
	Required     *bool    `json:"required,omitempty"`
	Type         string   `json:"type,omitempty"`
	Unique       *bool    `json:"unique,omitempty"`
	Update       *bool    `json:"update,omitempty"`
	ValidChars   string   `json:"validChars,omitempty"`
}
