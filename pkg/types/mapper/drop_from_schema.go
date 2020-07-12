package mapper

import "github.com/rancher/norman/types/mapper"

// DropFromSchema This mapper differs from the existing drop mapper in that
// it does not remove the field if it is present, only removing the field from
// the schema. This is so that fields that must be present for formatters and
// stores will be available, but not shown on the schema
type DropFromSchema struct {
	mapper.Drop
}

func NewDropFromSchema(name string) *DropFromSchema {
	return &DropFromSchema{
		mapper.Drop{
			Field: name,
		},
	}
}

func (d DropFromSchema) FromInternal(data map[string]interface{}) {
	// Do nothing
}
