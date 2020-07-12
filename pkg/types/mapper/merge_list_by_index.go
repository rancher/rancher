package mapper

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/mapper"
)

func NewMergeListByIndexMapper(From, To string, Ignores ...string) *MergeListByIndexMapper {
	rtn := MergeListByIndexMapper{
		From:       From,
		To:         To,
		Ignore:     make(map[string]struct{}),
		fromFields: []string{},
	}
	for _, Ignore := range Ignores {
		rtn.Ignore[Ignore] = struct{}{}
	}
	return &rtn
}

type MergeListByIndexMapper struct {
	From       string
	To         string
	Ignore     map[string]struct{}
	fromFields []string
}

func (m *MergeListByIndexMapper) FromInternal(data map[string]interface{}) {
	fromObj, ok := data[m.From]
	if !ok {
		return
	}
	toObj, ok := data[m.To]
	if !ok {
		return
	}
	fromList := convert.ToMapSlice(fromObj)
	toList := convert.ToMapSlice(toObj)
	for i := 0; i < len(fromList) && i < len(toList); i++ {
		fromItem := fromList[i]
		toItem := toList[i]
		for key, value := range fromItem {
			if _, ignore := m.Ignore[key]; ignore {
				continue
			}
			toItem[key] = value
		}
	}
	delete(data, m.From)
}

func (m *MergeListByIndexMapper) ToInternal(data map[string]interface{}) error {
	toObj, ok := data[m.To]
	if !ok {
		return nil
	}
	if _, ok = data[m.From]; ok {
		return fmt.Errorf("field %s should not exist", m.From)
	}

	toList := convert.ToMapSlice(toObj)
	var fromList []map[string]interface{}
	for _, toItem := range toList {
		obj := make(map[string]interface{})
		for _, field := range m.fromFields {
			value, ok := toItem[field]
			if !ok {
				continue
			}
			obj[field] = value
			if _, ok := m.Ignore[field]; !ok {
				delete(toItem, field)
			}
		}
		fromList = append(fromList, obj)
	}
	data[m.From] = fromList
	return nil
}

func (m *MergeListByIndexMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if err := mapper.ValidateField(m.From, schema); err != nil {
		return err
	}

	fromType := schema.ResourceFields[m.From].Type
	if !definition.IsArrayType(fromType) {
		return fmt.Errorf("type of field %s in schema %s is not array", m.From, schema.CodeName)
	}

	fromSchema := schemas.Schema(&schema.Version, definition.SubType(fromType))
	for field := range fromSchema.ResourceFields {
		m.fromFields = append(m.fromFields, field)
	}

	if err := mapper.ValidateField(m.To, schema); err != nil {
		return err
	}

	toType := schema.ResourceFields[m.To].Type
	if !definition.IsArrayType(toType) {
		return fmt.Errorf("type of field %s in schema %s is not array", m.To, schema.CodeName)
	}

	delete(schema.ResourceFields, m.From)
	return nil
}
