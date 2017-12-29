package types

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"net/http"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/slice"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	namespacedType = reflect.TypeOf(Namespaced{})
	resourceType   = reflect.TypeOf(Resource{})
	typeType       = reflect.TypeOf(metav1.TypeMeta{})
	metaType       = reflect.TypeOf(metav1.ObjectMeta{})
	blacklistNames = map[string]bool{
		"links":   true,
		"actions": true,
	}
)

func (s *Schemas) TypeName(name string, obj interface{}) *Schemas {
	s.typeNames[reflect.TypeOf(obj)] = name
	return s
}

func (s *Schemas) getTypeName(t reflect.Type) string {
	if name, ok := s.typeNames[t]; ok {
		return name
	}
	return convert.LowerTitle(t.Name())
}

func (s *Schemas) AddMapperForType(version *APIVersion, obj interface{}, mapper ...Mapper) *Schemas {
	if len(mapper) == 0 {
		return s
	}

	t := reflect.TypeOf(obj)
	typeName := s.getTypeName(t)
	if len(mapper) == 1 {
		return s.AddMapper(version, typeName, mapper[0])
	}
	return s.AddMapper(version, typeName, Mappers(mapper))
}

func (s *Schemas) MustImport(version *APIVersion, obj interface{}, externalOverrides ...interface{}) *Schemas {
	//TODO: remove
	logrus.SetLevel(logrus.DebugLevel)
	if _, err := s.Import(version, obj, externalOverrides...); err != nil {
		panic(err)
	}
	return s
}

func (s *Schemas) MustImportAndCustomize(version *APIVersion, obj interface{}, f func(*Schema), externalOverrides ...interface{}) *Schemas {
	return s.MustImport(version, obj, externalOverrides...).
		MustCustomizeType(version, obj, f)
}

func (s *Schemas) Import(version *APIVersion, obj interface{}, externalOverrides ...interface{}) (*Schema, error) {
	var types []reflect.Type
	for _, override := range externalOverrides {
		types = append(types, reflect.TypeOf(override))
	}

	return s.importType(version, reflect.TypeOf(obj), types...)
}

func (s *Schemas) newSchemaFromType(version *APIVersion, t reflect.Type, typeName string) (*Schema, error) {
	schema := &Schema{
		ID:             typeName,
		Version:        *version,
		CodeName:       t.Name(),
		PkgName:        t.PkgPath(),
		ResourceFields: map[string]Field{},
	}

	if err := s.readFields(schema, t); err != nil {
		return nil, err
	}

	return schema, nil
}

func (s *Schemas) setupFilters(schema *Schema) {
	if !slice.ContainsString(schema.CollectionMethods, http.MethodGet) {
		return
	}
	for fieldName, field := range schema.ResourceFields {
		var mods []ModifierType
		switch field.Type {
		case "enum":
			mods = []ModifierType{ModifierEQ, ModifierNE, ModifierIn, ModifierNotIn}
		case "string":
			mods = []ModifierType{ModifierEQ, ModifierNE, ModifierIn, ModifierNotIn}
		case "int":
			mods = []ModifierType{ModifierEQ, ModifierNE, ModifierIn, ModifierNotIn}
		case "boolean":
			mods = []ModifierType{ModifierEQ, ModifierNE}
		default:
			if definition.IsReferenceType(field.Type) {
				mods = []ModifierType{ModifierEQ, ModifierNE, ModifierIn, ModifierNotIn}
			}
		}

		if len(mods) > 0 {
			if schema.CollectionFilters == nil {
				schema.CollectionFilters = map[string]Filter{}
			}
			schema.CollectionFilters[fieldName] = Filter{
				Modifiers: mods,
			}
		}
	}
}

func (s *Schemas) MustCustomizeType(version *APIVersion, obj interface{}, f func(*Schema)) *Schemas {
	name := s.getTypeName(reflect.TypeOf(obj))
	schema := s.Schema(version, name)
	if schema == nil {
		panic("Failed to find schema " + name)
	}

	f(schema)

	if schema.SubContext != "" {
		s.schemasBySubContext[schema.SubContext] = schema
	}

	return s
}

func (s *Schemas) importType(version *APIVersion, t reflect.Type, overrides ...reflect.Type) (*Schema, error) {
	typeName := s.getTypeName(t)

	existing := s.Schema(version, typeName)
	if existing != nil {
		return existing, nil
	}

	logrus.Debugf("Inspecting schema %s for %v", typeName, t)

	schema, err := s.newSchemaFromType(version, t, typeName)
	if err != nil {
		return nil, err
	}

	mappers := s.mapper(&schema.Version, schema.ID)
	if s.DefaultMappers != nil {
		if schema.CanList() {
			mappers = append(s.DefaultMappers(), mappers...)
		}
	}
	if s.DefaultPostMappers != nil {
		mappers = append(mappers, s.DefaultPostMappers()...)
	}

	if len(mappers) > 0 {
		copy, err := s.newSchemaFromType(version, t, typeName)
		if err != nil {
			return nil, err
		}
		schema.InternalSchema = copy
	}

	for _, override := range overrides {
		if err := s.readFields(schema, override); err != nil {
			return nil, err
		}
	}

	mapper := &typeMapper{
		Mappers: mappers,
		root:    schema.CanList(),
	}

	if err := mapper.ModifySchema(schema, s); err != nil {
		return nil, err
	}

	s.setupFilters(schema)

	schema.Mapper = mapper
	s.AddSchema(*schema)

	return s.Schema(&schema.Version, schema.ID), s.Err()
}

func jsonName(f reflect.StructField) string {
	return strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
}

func (s *Schemas) readFields(schema *Schema, t reflect.Type) error {
	if t == resourceType {
		schema.CollectionMethods = []string{"GET", "POST"}
		schema.ResourceMethods = []string{"GET", "PUT", "DELETE"}
	}

	hasType := false
	hasMeta := false

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.PkgPath != "" {
			// unexported field
			continue
		}

		jsonName := jsonName(field)

		if jsonName == "-" {
			continue
		}

		if field.Anonymous && jsonName == "" && field.Type == typeType {
			hasType = true
		}

		if field.Anonymous && jsonName == "metadata" && field.Type == metaType {
			hasMeta = true
		}

		if field.Anonymous && jsonName == "" {
			t := field.Type
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			if t.Kind() == reflect.Struct {
				if t == namespacedType {
					schema.Scope = NamespaceScope
				}
				if err := s.readFields(schema, t); err != nil {
					return err
				}
			}
			continue
		}

		fieldName := jsonName
		if fieldName == "" {
			fieldName = convert.LowerTitle(field.Name)
			if strings.HasSuffix(fieldName, "ID") {
				fieldName = strings.TrimSuffix(fieldName, "ID") + "Id"
			}
		}

		if blacklistNames[fieldName] {
			logrus.Debugf("Ignoring blacklisted field %s.%s for %v", schema.ID, fieldName, field)
			continue
		}

		logrus.Debugf("Inspecting field %s.%s for %v", schema.ID, fieldName, field)

		schemaField := Field{
			Create:   true,
			Update:   true,
			Nullable: true,
			CodeName: field.Name,
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			schemaField.Nullable = true
			fieldType = fieldType.Elem()
		}

		if err := applyTag(&field, &schemaField); err != nil {
			return err
		}

		if schemaField.Type == "" {
			inferedType, err := s.determineSchemaType(&schema.Version, fieldType)
			if err != nil {
				return err
			}
			schemaField.Type = inferedType
		}

		logrus.Debugf("Setting field %s.%s: %#v", schema.ID, fieldName, schemaField)
		schema.ResourceFields[fieldName] = schemaField
	}

	if hasType && hasMeta {
		schema.CollectionMethods = []string{"GET", "POST"}
		schema.ResourceMethods = []string{"GET", "PUT", "DELETE"}
	}

	return nil
}

func applyTag(structField *reflect.StructField, field *Field) error {
	for _, part := range strings.Split(structField.Tag.Get("norman"), ",") {
		if part == "" {
			continue
		}

		var err error
		key, value := getKeyValue(part)

		switch key {
		case "type":
			field.Type = value
		case "codeName":
			field.CodeName = value
		case "default":
			field.Default = value
		case "nullabled":
			field.Nullable = true
		case "nocreate":
			field.Create = false
		case "writeOnly":
			field.WriteOnly = true
		case "required":
			field.Required = true
		case "noupdate":
			field.Update = false
		case "minLength":
			field.MinLength, err = toInt(value, structField)
		case "maxLength":
			field.MaxLength, err = toInt(value, structField)
		case "min":
			field.Min, err = toInt(value, structField)
		case "max":
			field.Max, err = toInt(value, structField)
		case "options":
			field.Options = split(value)
			if field.Type == "" {
				field.Type = "enum"
			}
		case "validChars":
			field.ValidChars = value
		case "invalidChars":
			field.InvalidChars = value
		default:
			return fmt.Errorf("invalid tag %s on field %s", key, structField.Name)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func toInt(value string, structField *reflect.StructField) (*int64, error) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number on field %s: %v", structField.Name, err)
	}
	return &i, nil
}

func split(input string) []string {
	result := []string{}
	for _, i := range strings.Split(input, "|") {
		for _, part := range strings.Split(i, " ") {
			part = strings.TrimSpace(part)
			if len(part) > 0 {
				result = append(result, part)
			}
		}
	}

	return result
}

func getKeyValue(input string) (string, string) {
	var (
		key, value string
	)
	parts := strings.SplitN(input, "=", 2)
	key = parts[0]
	if len(parts) > 1 {
		value = parts[1]
	}

	return key, value
}

func (s *Schemas) determineSchemaType(version *APIVersion, t reflect.Type) (string, error) {
	switch t.Kind() {
	case reflect.Uint8:
		return "byte", nil
	case reflect.Bool:
		return "boolean", nil
	case reflect.Int:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		return "int", nil
	case reflect.Interface:
		return "json", nil
	case reflect.Map:
		subType, err := s.determineSchemaType(version, t.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("map[%s]", subType), nil
	case reflect.Slice:
		subType, err := s.determineSchemaType(version, t.Elem())
		if err != nil {
			return "", err
		}
		if subType == "byte" {
			return "base64", nil
		}
		return fmt.Sprintf("array[%s]", subType), nil
	case reflect.String:
		return "string", nil
	case reflect.Struct:
		if t.Name() == "Time" {
			return "date", nil
		}
		if t.Name() == "IntOrString" {
			return "intOrString", nil
		}
		if t.Name() == "Quantity" {
			return "string", nil
		}
		schema, err := s.importType(version, t)
		if err != nil {
			return "", err
		}
		return schema.ID, nil
	default:
		return "", fmt.Errorf("unknown type kind %s", t.Kind())
	}

}
