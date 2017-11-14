package types

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/rancher/norman/name"
	"github.com/rancher/norman/types/convert"
)

type SchemaCollection struct {
	Data []Schema
}

type SchemaInitFunc func(*Schemas) *Schemas

type MappersFactory func() []Mapper

type Schemas struct {
	schemasByPath       map[string]map[string]*Schema
	schemasBySubContext map[string]*Schema
	mappers             map[string]map[string][]Mapper
	DefaultMappers      MappersFactory
	DefaultPostMappers  MappersFactory
	versions            []APIVersion
	schemas             []*Schema
	errors              []error
}

func NewSchemas() *Schemas {
	return &Schemas{
		schemasByPath:       map[string]map[string]*Schema{},
		schemasBySubContext: map[string]*Schema{},
		mappers:             map[string]map[string][]Mapper{},
	}
}

func (s *Schemas) Init(initFunc SchemaInitFunc) *Schemas {
	return initFunc(s)
}

func (s *Schemas) Err() error {
	return NewErrors(s.errors)
}

func (s *Schemas) SubContext(subContext string) *Schema {
	return s.schemasBySubContext[subContext]
}

func (s *Schemas) SubContextSchemas() map[string]*Schema {
	return s.schemasBySubContext
}

func (s *Schemas) AddSchemas(schema *Schemas) *Schemas {
	for _, schema := range schema.Schemas() {
		s.AddSchema(*schema)
	}
	return s
}

func (s *Schemas) AddSchema(schema Schema) *Schemas {
	schema.Type = "/meta/schemas/schema"
	if schema.ID == "" {
		s.errors = append(s.errors, fmt.Errorf("ID is not set on schema: %v", schema))
		return s
	}
	if schema.Version.Path == "" || schema.Version.Version == "" {
		s.errors = append(s.errors, fmt.Errorf("version is not set on schema: %s", schema.ID))
		return s
	}
	if schema.PluralName == "" {
		schema.PluralName = name.GuessPluralName(schema.ID)
	}
	if schema.CodeName == "" {
		schema.CodeName = convert.Capitalize(schema.ID)
	}
	if schema.CodeNamePlural == "" {
		schema.CodeNamePlural = name.GuessPluralName(schema.CodeName)
	}
	if schema.BaseType == "" {
		schema.BaseType = schema.ID
	}

	schemas, ok := s.schemasByPath[schema.Version.Path]
	if !ok {
		schemas = map[string]*Schema{}
		s.schemasByPath[schema.Version.Path] = schemas
		s.versions = append(s.versions, schema.Version)
	}

	if _, ok := schemas[schema.ID]; !ok {
		schemas[schema.ID] = &schema
		s.schemas = append(s.schemas, &schema)
	}

	if schema.SubContext != "" {
		s.schemasBySubContext[schema.SubContext] = &schema
	}

	return s
}

func (s *Schemas) AddMapper(version *APIVersion, schemaID string, mapper Mapper) *Schemas {
	mappers, ok := s.mappers[version.Path]
	if !ok {
		mappers = map[string][]Mapper{}
		s.mappers[version.Path] = mappers
	}

	mappers[schemaID] = append(mappers[schemaID], mapper)
	return s
}

func (s *Schemas) SchemasForVersion(version APIVersion) map[string]*Schema {
	return s.schemasByPath[version.Path]
}

func (s *Schemas) Versions() []APIVersion {
	return s.versions
}

func (s *Schemas) Schemas() []*Schema {
	return s.schemas
}

func (s *Schemas) mapper(version *APIVersion, name string) []Mapper {
	var (
		path string
	)

	if strings.Contains(name, "/") {
		idx := strings.LastIndex(name, "/")
		path = name[0:idx]
		name = name[idx+1:]
	} else if version != nil {
		path = version.Path
	} else {
		path = "core"
	}

	mappers, ok := s.mappers[path]
	if !ok {
		return nil
	}

	mapper := mappers[name]
	if mapper != nil {
		return mapper
	}

	return nil
}

func (s *Schemas) Schema(version *APIVersion, name string) *Schema {
	var (
		path string
	)

	if strings.Contains(name, "/schemas/") {
		parts := strings.SplitN(name, "/schemas/", 2)
		path = parts[0]
		name = parts[1]
	} else if version != nil {
		path = version.Path
	} else {
		path = "core"
	}

	schemas, ok := s.schemasByPath[path]
	if !ok {
		return nil
	}

	schema := schemas[name]
	if schema != nil {
		return schema
	}

	for _, check := range schemas {
		if strings.EqualFold(check.ID, name) || strings.EqualFold(check.PluralName, name) {
			return check
		}
	}

	return nil
}

type multiErrors struct {
	errors []error
}

func NewErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	}
	return &multiErrors{
		errors: errors,
	}
}

func (m *multiErrors) Error() string {
	buf := bytes.NewBuffer(nil)
	for _, err := range m.errors {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(err.Error())
	}

	return buf.String()
}
