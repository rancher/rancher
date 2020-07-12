package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type WorkloadAnnotations struct {
}

func (n WorkloadAnnotations) FromInternal(data map[string]interface{}) {
	v, ok := values.RemoveValue(data, "workloadAnnotations", "field.cattle.io/publicEndpoints")
	if ok {
		if _, ok := data["annotations"]; !ok {
			data["annotations"] = map[string]interface{}{}
		}
		annotations := convert.ToMapInterface(data["annotations"])
		annotations["field.cattle.io/publicEndpoints"] = v
	}
}

func (n WorkloadAnnotations) ToInternal(data map[string]interface{}) error {
	return nil
}

func (n WorkloadAnnotations) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
