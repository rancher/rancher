package mappers

import (
	"github.com/rancher/wrangler/pkg/data"
	types "github.com/rancher/wrangler/pkg/schemas"
)

type AliasField struct {
	Field string
	Names []string
}

func NewAlias(field string, names ...string) types.Mapper {
	return AliasField{
		Field: field,
		Names: names,
	}
}

func (d AliasField) FromInternal(data data.Object) {
}

func (d AliasField) ToInternal(data data.Object) error {
	for _, name := range d.Names {
		if v, ok := data[name]; ok {
			delete(data, name)
			data[d.Field] = v
		}
	}
	return nil
}

func (d AliasField) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	for _, name := range d.Names {
		schema.ResourceFields[name] = types.Field{}
	}

	return ValidateField(d.Field, schema)
}
