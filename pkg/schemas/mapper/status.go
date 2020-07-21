package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/schemas/status"
)

type Status struct {
}

func (s Status) FromInternal(data map[string]interface{}) {
	status.Set(data)
}

func (s Status) ToInternal(data map[string]interface{}) error {
	return nil
}

func (s Status) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	_, hasSpec := schema.ResourceFields["spec"]
	_, hasStatus := schema.ResourceFields["status"]

	if !hasSpec || !hasStatus {
		return nil
	}

	schema.ResourceFields["state"] = types.Field{
		CodeName: "State",
		Type:     "string",
	}
	schema.ResourceFields["transitioning"] = types.Field{
		CodeName: "Transitioning",
		Type:     "enum",
		Options: []string{
			"yes",
			"no",
			"error",
		},
	}
	schema.ResourceFields["transitioningMessage"] = types.Field{
		CodeName: "TransitioningMessage",
		Type:     "string",
	}
	return nil
}
