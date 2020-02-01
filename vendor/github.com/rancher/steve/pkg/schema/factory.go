package schema

import (
	"fmt"
	"net/http"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/table"
	"github.com/rancher/steve/pkg/schemaserver/builtin"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas"
	"k8s.io/apiserver/pkg/authentication/user"
)

func newSchemas() (*types.APISchemas, error) {
	apiSchemas := types.EmptyAPISchemas()
	if err := apiSchemas.AddSchemas(builtin.Schemas); err != nil {
		return nil, err
	}
	apiSchemas.InternalSchemas.DefaultMapper = func() schemas.Mapper {
		return newDefaultMapper()
	}

	return apiSchemas, nil
}

func (c *Collection) Schemas(user user.Info) (*types.APISchemas, error) {
	access := c.as.AccessFor(user)
	return c.schemasForSubject(access)
}

func (c *Collection) schemasForSubject(access *accesscontrol.AccessSet) (*types.APISchemas, error) {
	result, err := newSchemas()
	if err != nil {
		return nil, err
	}

	if err := result.AddSchemas(c.baseSchema); err != nil {
		return nil, err
	}

	for _, s := range c.schemas {
		gr := attributes.GR(s)

		if gr.Resource == "" {
			if err := result.AddSchema(*s); err != nil {
				return nil, err
			}
			continue
		}

		verbs := attributes.Verbs(s)
		verbAccess := accesscontrol.AccessListByVerb{}

		for _, verb := range verbs {
			a := access.AccessListFor(verb, gr)
			if len(a) > 0 {
				verbAccess[verb] = a
			}
		}

		if len(verbAccess) == 0 {
			continue
		}

		s = s.DeepCopy()
		attributes.SetAccess(s, verbAccess)
		if verbAccess.AnyVerb("list", "get") {
			s.ResourceMethods = append(s.ResourceMethods, http.MethodGet)
			s.CollectionMethods = append(s.CollectionMethods, http.MethodGet)
		}
		if verbAccess.AnyVerb("delete") {
			s.ResourceMethods = append(s.ResourceMethods, http.MethodDelete)
		}
		if verbAccess.AnyVerb("update") {
			s.ResourceMethods = append(s.ResourceMethods, http.MethodPut)
			s.ResourceMethods = append(s.ResourceMethods, http.MethodPatch)
		}
		if verbAccess.AnyVerb("create") {
			s.CollectionMethods = append(s.CollectionMethods, http.MethodPost)
		}

		c.applyTemplates(result, s)

		if err := result.AddSchema(*s); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (c *Collection) applyTemplates(schemas *types.APISchemas, schema *types.APISchema) {
	templates := []*Template{
		c.templates[schema.ID],
		c.templates[fmt.Sprintf("%s/%s", attributes.Group(schema), attributes.Kind(schema))],
		c.templates[""],
	}

	for _, t := range templates {
		if t == nil {
			continue
		}
		if t.Mapper != nil {
			schemas.InternalSchemas.AddMapper(schema.ID, t.Mapper)
		}
		if schema.Formatter == nil {
			schema.Formatter = t.Formatter
		}
		if schema.Store == nil {
			if t.StoreFactory == nil {
				schema.Store = t.Store
			} else {
				schema.Store = t.StoreFactory(templates[2].Store)
			}
		}
		if t.Customize != nil {
			t.Customize(schema)
		}
		if len(t.Columns) > 0 {
			schemas.InternalSchemas.AddMapper(schema.ID, table.NewColumns(t.ComputedColumns, t.Columns...))
		}
	}
}
