package types

import (
	"fmt"

	"github.com/rancher/norman/types/definition"
)

type Mapper interface {
	FromInternal(data map[string]interface{})
	ToInternal(data map[string]interface{})
	ModifySchema(schema *Schema, schemas *Schemas) error
}

type Mappers []Mapper

func (m Mappers) FromInternal(data map[string]interface{}) {
	for _, mapper := range m {
		mapper.FromInternal(data)
	}
}

func (m Mappers) ToInternal(data map[string]interface{}) {
	for i := len(m) - 1; i >= 0; i-- {
		m[i].ToInternal(data)
	}
}

func (m Mappers) ModifySchema(schema *Schema, schemas *Schemas) error {
	for _, mapper := range m {
		if err := mapper.ModifySchema(schema, schemas); err != nil {
			return err
		}
	}
	return nil
}

type typeMapper struct {
	Mappers         []Mapper
	typeName        string
	subSchemas      map[string]*Schema
	subArraySchemas map[string]*Schema
}

func (t *typeMapper) FromInternal(data map[string]interface{}) {
	for fieldName, schema := range t.subSchemas {
		if schema.Mapper == nil {
			continue
		}
		fieldData, _ := data[fieldName].(map[string]interface{})
		schema.Mapper.FromInternal(fieldData)
	}

	for fieldName, schema := range t.subArraySchemas {
		if schema.Mapper == nil {
			continue
		}
		datas, _ := data[fieldName].([]interface{})
		for _, fieldData := range datas {
			mapFieldData, _ := fieldData.(map[string]interface{})
			schema.Mapper.FromInternal(mapFieldData)
		}
	}

	Mappers(t.Mappers).FromInternal(data)

	if data != nil {
		if _, ok := data["type"]; !ok {
			data["type"] = t.typeName
		}
		name, _ := data["name"].(string)
		namespace, _ := data["namespaceId"].(string)

		if _, ok := data["id"]; !ok {
			if name != "" {
				if namespace == "" {
					data["id"] = name
				} else {
					data["id"] = namespace + ":" + name
				}
			}
		}
	}
}

func (t *typeMapper) ToInternal(data map[string]interface{}) {
	Mappers(t.Mappers).ToInternal(data)

	for fieldName, schema := range t.subArraySchemas {
		if schema.Mapper == nil {
			continue
		}
		datas, _ := data[fieldName].([]map[string]interface{})
		for _, fieldData := range datas {
			schema.Mapper.ToInternal(fieldData)
		}
	}

	for fieldName, schema := range t.subSchemas {
		if schema.Mapper == nil {
			continue
		}
		fieldData, _ := data[fieldName].(map[string]interface{})
		schema.Mapper.ToInternal(fieldData)
	}
}

func (t *typeMapper) ModifySchema(schema *Schema, schemas *Schemas) error {
	t.subSchemas = map[string]*Schema{}
	t.subArraySchemas = map[string]*Schema{}
	t.typeName = fmt.Sprintf("%s/schemas/%s", schema.Version.Path, schema.ID)

	mapperSchema := schema
	if schema.InternalSchema != nil {
		mapperSchema = schema.InternalSchema
	}
	for name, field := range mapperSchema.ResourceFields {
		fieldType := field.Type
		targetMap := t.subSchemas
		if definition.IsArrayType(fieldType) {
			fieldType = definition.SubType(fieldType)
			targetMap = t.subArraySchemas
		}

		schema := schemas.Schema(&schema.Version, fieldType)
		if schema != nil {
			targetMap[name] = schema
		}
	}

	return Mappers(t.Mappers).ModifySchema(schema, schemas)
}
