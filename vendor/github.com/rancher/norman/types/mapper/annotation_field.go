package mapper

import (
	"encoding/json"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type AnnotationField struct {
	Field            string
	Object           bool
	IgnoreDefinition bool
}

func (e AnnotationField) FromInternal(data map[string]interface{}) {
	v, ok := values.RemoveValue(data, "annotations", "field.cattle.io/"+e.Field)
	if ok {
		if e.Object {
			data := map[string]interface{}{}
			//ignore error
			if err := json.Unmarshal([]byte(convert.ToString(v)), &data); err == nil {
				v = data
			}
		}

		data[e.Field] = v
	}
}

func (e AnnotationField) ToInternal(data map[string]interface{}) {
	v, ok := data[e.Field]
	if ok {
		if e.Object {
			if bytes, err := json.Marshal(v); err == nil {
				v = string(bytes)
			}
		}
		values.PutValue(data, v, "annotations", "field.cattle.io/"+e.Field)
	}
}

func (e AnnotationField) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if e.IgnoreDefinition {
		return nil
	}
	return ValidateField(e.Field, schema)
}
