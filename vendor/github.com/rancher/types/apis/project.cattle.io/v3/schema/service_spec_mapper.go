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
		data["clusterIp"] = "None"
	} else {
		data["type"] = "ExternalName"
		data["clusterIp"] = ""
	}
}

func (e ServiceSpecMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
