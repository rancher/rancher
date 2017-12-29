package schema

import (
	"github.com/rancher/norman/types"
)

type ServiceSpecMapper struct {
}

func (e ServiceSpecMapper) FromInternal(data map[string]interface{}) {
}

func (e ServiceSpecMapper) ToInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	data["clusterIp"] = "None"
	data["type"] = "ClusterIP"
}

func (e ServiceSpecMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
