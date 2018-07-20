package schema

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type ServiceSpecMapper struct {
}

func (e ServiceSpecMapper) FromInternal(data map[string]interface{}) {
}

func (e ServiceSpecMapper) ToInternal(data map[string]interface{}) error {
	if data == nil {
		return nil
	}

	if convert.IsAPIObjectEmpty(data["hostname"]) {
		data["type"] = "ClusterIP"
		data["clusterIP"] = "None"
	} else {
		data["type"] = "ExternalName"
		data["clusterIP"] = ""
	}

	return nil
}

func (e ServiceSpecMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
