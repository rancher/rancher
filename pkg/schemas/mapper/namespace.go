package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/mapper"
)

type NamespaceIDMapper struct {
	Move *mapper.Move
}

func (n *NamespaceIDMapper) FromInternal(data map[string]interface{}) {
	if n.Move != nil {
		n.Move.FromInternal(data)
	}
}

func (n *NamespaceIDMapper) ToInternal(data map[string]interface{}) error {
	if n.Move != nil {
		return n.Move.ToInternal(data)
	}
	return nil
}

func (n *NamespaceIDMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	field, ok := schema.ResourceFields["namespace"]
	if !ok {
		return nil
	}

	field.Type = "reference[/v3/clusters/schemas/namespace]"
	field.Required = true
	field.Update = false
	schema.ResourceFields["namespace"] = field

	n.Move = &mapper.Move{
		From: "namespace",
		To:   "namespaceId",
	}

	return n.Move.ModifySchema(schema, schemas)
}
