package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/mapper"
)

type Creator struct {
	m types.Mapper
}

func (c *Creator) FromInternal(data map[string]interface{}) {
	if c.m != nil {
		c.m.FromInternal(data)
	}
}

func (c *Creator) ToInternal(data map[string]interface{}) error {
	if c.m != nil {
		return c.m.ToInternal(data)
	}
	return nil
}

func (c *Creator) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if schema.CanList(nil) == nil && schema.CanCreate(nil) == nil {
		schema.ResourceFields["creatorId"] = types.Field{
			Type:     "reference[/v3/schemas/user]",
			CodeName: "CreatorID",
		}
		c.m = &mapper.AnnotationField{Field: "creatorId"}
		return c.m.ModifySchema(schema, schemas)
	}
	return nil
}
