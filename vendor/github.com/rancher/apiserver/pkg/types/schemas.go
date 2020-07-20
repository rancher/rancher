package types

import (
	"strings"
	"sync"

	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
)

type APISchemas struct {
	sync.Mutex
	InternalSchemas *schemas.Schemas
	Schemas         map[string]*APISchema
	index           map[string]*APISchema
}

func EmptyAPISchemas() *APISchemas {
	return &APISchemas{
		InternalSchemas: schemas.EmptySchemas(),
		Schemas:         map[string]*APISchema{},
		index:           map[string]*APISchema{},
	}
}

func (a *APISchemas) ShallowCopy() *APISchemas {
	a.Lock()
	defer a.Unlock()
	result := &APISchemas{
		InternalSchemas: a.InternalSchemas,
		Schemas:         map[string]*APISchema{},
		index:           map[string]*APISchema{},
	}
	for k, v := range a.Schemas {
		result.Schemas[k] = v
	}
	for k, v := range a.index {
		result.index[k] = v
	}
	return result
}

func (a *APISchemas) MustAddSchema(obj APISchema) *APISchemas {
	err := a.AddSchema(obj)
	if err != nil {
		logrus.Fatalf("failed to add schema: %v", err)
	}
	return a
}

func (a *APISchemas) addInternalSchema(schema *schemas.Schema) *APISchema {
	apiSchema := &APISchema{
		Schema: schema,
	}
	a.Schemas[schema.ID] = apiSchema
	a.addToIndex(apiSchema)

	for _, f := range schema.ResourceFields {
		if subType := a.InternalSchemas.Schema(f.Type); subType == nil {
			continue
		} else if _, ok := a.Schemas[subType.ID]; !ok {
			a.addInternalSchema(subType)
		}
	}

	return apiSchema
}

func (a *APISchemas) Import(obj interface{}) (*APISchema, error) {
	a.Lock()
	defer a.Unlock()
	schema, err := a.InternalSchemas.Import(obj)
	if err != nil {
		return nil, err
	}
	apiSchema := a.addInternalSchema(schema)
	return apiSchema, nil
}

func (a *APISchemas) MustImportAndCustomize(obj interface{}, f func(*APISchema)) {
	a.Lock()
	defer a.Unlock()
	schema, err := a.InternalSchemas.Import(obj)
	if err != nil {
		panic(err)
	}
	apiSchema := a.addInternalSchema(schema)
	if f != nil {
		f(apiSchema)
	}
}

func (a *APISchemas) MustAddSchemas(schemas *APISchemas) *APISchemas {
	if err := a.AddSchemas(schemas); err != nil {
		logrus.Fatalf("failed to add schemas: %v", err)
	}
	return a
}

func (a *APISchemas) AddSchemas(schema *APISchemas) error {
	for _, schema := range schema.Schemas {
		if err := a.AddSchema(*schema); err != nil {
			return err
		}
	}
	return nil
}

func (a *APISchemas) addToIndex(schema *APISchema) {
	a.index[strings.ToLower(schema.ID)] = schema
	a.index[strings.ToLower(schema.PluralName)] = schema
}

func (a *APISchemas) AddSchema(schema APISchema) error {
	a.Lock()
	defer a.Unlock()
	if err := a.InternalSchemas.AddSchema(*schema.Schema); err != nil {
		return err
	}
	schema.Schema = a.InternalSchemas.Schema(schema.ID)
	a.Schemas[schema.ID] = &schema
	a.addToIndex(&schema)
	return nil
}

func (a *APISchemas) LookupSchema(name string) *APISchema {
	a.Lock()
	defer a.Unlock()
	s, ok := a.Schemas[name]
	if ok {
		return s
	}
	if s, ok := a.index[strings.ToLower(name)]; ok {
		// if schema is removed it may be left in the index
		return a.Schemas[s.ID]
	}
	return nil
}
