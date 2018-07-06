package schema

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type ServiceSpecMapper struct {
}

func (e ServiceSpecMapper) FromInternal(data map[string]interface{}) {
}

func (e ServiceSpecMapper) ToInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	if convert.IsEmpty(data["hostname"]) {
		data["type"] = "ClusterIP"
		data["clusterIP"] = "None"
	} else {
		data["type"] = "ExternalName"
		data["clusterIP"] = ""
	}
}

func (e ServiceSpecMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
