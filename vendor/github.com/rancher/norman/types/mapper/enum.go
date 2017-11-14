package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type Enum struct {
	Field  string
	Values map[string][]string
}

func (e Enum) FromInternal(data map[string]interface{}) {
	v, ok := data[e.Field]
	if !ok {
		return
	}

	str := convert.ToString(v)

	mapping, ok := e.Values[str]
	if ok {
		data[e.Field] = mapping[0]
	} else {
		data[e.Field] = v
	}
}

func (e Enum) ToInternal(data map[string]interface{}) {
	v, ok := data[e.Field]
	if !ok {
		return
	}

	str := convert.ToString(v)
	for newValue, values := range e.Values {
		for _, value := range values {
			if str == value {
				data[e.Field] = newValue
				return
			}
		}
	}
}

func (e Enum) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return validateField(e.Field, schema)
}
