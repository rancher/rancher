package client

const (
	QuestionType              = "question"
	QuestionFieldDefault      = "default"
	QuestionFieldDescription  = "description"
	QuestionFieldGroup        = "group"
	QuestionFieldInvalidChars = "invalidChars"
	QuestionFieldLabel        = "label"
	QuestionFieldMax          = "max"
	QuestionFieldMaxLength    = "maxLength"
	QuestionFieldMin          = "min"
	QuestionFieldMinLength    = "minLength"
	QuestionFieldOptions      = "options"
	QuestionFieldRequired     = "required"
	QuestionFieldType         = "type"
	QuestionFieldValidChars   = "validChars"
	QuestionFieldVariable     = "variable"
)

type Question struct {
	Default      string   `json:"default,omitempty"`
	Description  string   `json:"description,omitempty"`
	Group        string   `json:"group,omitempty"`
	InvalidChars string   `json:"invalidChars,omitempty"`
	Label        string   `json:"label,omitempty"`
	Max          *int64   `json:"max,omitempty"`
	MaxLength    *int64   `json:"maxLength,omitempty"`
	Min          *int64   `json:"min,omitempty"`
	MinLength    *int64   `json:"minLength,omitempty"`
	Options      []string `json:"options,omitempty"`
	Required     bool     `json:"required,omitempty"`
	Type         string   `json:"type,omitempty"`
	ValidChars   string   `json:"validChars,omitempty"`
	Variable     string   `json:"variable,omitempty"`
}
