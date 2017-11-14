package mapper

import (
	"github.com/rancher/norman/types"
)

type ReadOnly struct {
	Field    string
	Optional bool
}

func (r ReadOnly) FromInternal(data map[string]interface{}) {
}

func (r ReadOnly) ToInternal(data map[string]interface{}) {
}

func (r ReadOnly) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if r.Field == "*" {
		for name, field := range schema.ResourceFields {
			field.Create = false
			field.Update = false
			schema.ResourceFields[name] = field
		}
		return nil
	}

	if err := validateField(r.Field, schema); err != nil {
		if r.Optional {
			return nil
		}
		return err
	}

	field := schema.ResourceFields[r.Field]
	field.Create = false
	field.Update = false
	schema.ResourceFields[r.Field] = field

	return nil
}
