package mappers

import (
	"strings"

	"github.com/rancher/wrangler/pkg/data"
	types "github.com/rancher/wrangler/pkg/schemas"
)

type Access struct {
	Fields   map[string]string
	Optional bool
}

func (e Access) FromInternal(data data.Object) {
}

func (e Access) ToInternal(data data.Object) error {
	return nil
}

func (e Access) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	for name, access := range e.Fields {
		if err := ValidateField(name, schema); err != nil {
			if e.Optional {
				continue
			}
			return err
		}

		field := schema.ResourceFields[name]
		field.Create = strings.Contains(access, "c")
		field.Update = strings.Contains(access, "u")
		field.WriteOnly = strings.Contains(access, "o")

		schema.ResourceFields[name] = field
	}
	return nil
}
