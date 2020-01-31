package mappers

import (
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	types "github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/definition"
)

type Move struct {
	Optional           bool
	From, To, CodeName string
	DestDefined        bool
	NoDeleteFromField  bool
}

func (m Move) FromInternal(d data.Object) {
	if v, ok := data.RemoveValue(d, strings.Split(m.From, "/")...); ok {
		data.PutValue(d, v, strings.Split(m.To, "/")...)
	}
}

func (m Move) ToInternal(d data.Object) error {
	if v, ok := data.RemoveValue(d, strings.Split(m.To, "/")...); ok {
		data.PutValue(d, v, strings.Split(m.From, "/")...)
	}
	return nil
}

func (m Move) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	fromSchema, _, fromField, ok, err := getField(s, schemas, m.From)
	if err != nil {
		return err
	}
	if !ok {
		if m.Optional {
			return nil
		}
		return fmt.Errorf("failed to find field %s on schema %s", m.From, s.ID)
	}

	toSchema, toFieldName, _, ok, err := getField(s, schemas, m.To)
	if err != nil {
		return err
	}
	_, ok = toSchema.ResourceFields[toFieldName]
	if ok && !strings.Contains(m.To, "/") && !m.DestDefined {
		return fmt.Errorf("field %s already exists on schema %s", m.To, s.ID)
	}

	if !m.NoDeleteFromField {
		delete(fromSchema.ResourceFields, m.From)
	}

	if !m.DestDefined {
		if m.CodeName == "" {
			fromField.CodeName = convert.Capitalize(toFieldName)
		} else {
			fromField.CodeName = m.CodeName
		}
		toSchema.ResourceFields[toFieldName] = fromField
	}

	return nil
}

func getField(schema *types.Schema, schemas *types.Schemas, target string) (*types.Schema, string, types.Field, bool, error) {
	parts := strings.Split(target, "/")
	for i, part := range parts {
		if i == len(parts)-1 {
			continue
		}

		fieldType := schema.ResourceFields[part].Type
		if definition.IsArrayType(fieldType) {
			fieldType = definition.SubType(fieldType)
		}
		subSchema := schemas.Schema(fieldType)
		if subSchema == nil {
			return nil, "", types.Field{}, false, fmt.Errorf("failed to find field or schema for %s on %s", part, schema.ID)
		}

		schema = subSchema
	}

	name := parts[len(parts)-1]
	f, ok := schema.ResourceFields[name]
	return schema, name, f, ok, nil
}
