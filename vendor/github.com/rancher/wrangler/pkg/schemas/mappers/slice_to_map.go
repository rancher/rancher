package mappers

import (
	"fmt"

	"github.com/rancher/wrangler/pkg/data"
	types "github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/definition"
)

type SliceToMap struct {
	Field string
	Key   string
}

func (s SliceToMap) FromInternal(data data.Object) {
	datas := data.Slice(s.Field)
	result := map[string]interface{}{}

	for _, item := range datas {
		name, _ := item[s.Key].(string)
		delete(item, s.Key)
		result[name] = item
	}

	if len(result) > 0 {
		data[s.Field] = result
	}
}

func (s SliceToMap) ToInternal(data data.Object) error {
	datas := data.Map(s.Field)
	var result []interface{}

	for name, item := range datas {
		mapItem, _ := item.(map[string]interface{})
		if mapItem != nil {
			mapItem[s.Key] = name
			result = append(result, mapItem)
		}
	}

	if len(result) > 0 {
		data[s.Field] = result
	} else if datas != nil {
		data[s.Field] = result
	}

	return nil
}

func (s SliceToMap) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	err := ValidateField(s.Field, schema)
	if err != nil {
		return err
	}

	subSchema, subFieldName, _, _, err := getField(schema, schemas, fmt.Sprintf("%s/%s", s.Field, s.Key))
	if err != nil {
		return err
	}

	field := schema.ResourceFields[s.Field]
	if !definition.IsArrayType(field.Type) {
		return fmt.Errorf("field %s on %s is not an array", s.Field, schema.ID)
	}

	field.Type = "map[" + definition.SubType(field.Type) + "]"
	schema.ResourceFields[s.Field] = field

	delete(subSchema.ResourceFields, subFieldName)

	return nil
}
