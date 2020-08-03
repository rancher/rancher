package schema

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/builtin"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"k8s.io/apiserver/pkg/authentication/user"
)

func newSchemas() (*types.APISchemas, error) {
	apiSchemas := types.EmptyAPISchemas()
	if err := apiSchemas.AddSchemas(builtin.Schemas); err != nil {
		return nil, err
	}

	return apiSchemas, nil
}

func (c *Collection) Schemas(user user.Info) (*types.APISchemas, error) {
	access := c.as.AccessFor(user)
	val, ok := c.cache.Get(access.ID)
	if ok {
		schemas, _ := val.(*types.APISchemas)
		return schemas, nil
	}

	schemas, err := c.schemasForSubject(access)
	if err != nil {
		return nil, err
	}

	c.cache.Add(access.ID, schemas, 24*time.Hour)
	return schemas, nil
}

func (c *Collection) schemasForSubject(access *accesscontrol.AccessSet) (*types.APISchemas, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

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
			if !attributes.Namespaced(s) {
				// trim out bad data where we are granted namespaced access to cluster scoped object
				result := accesscontrol.AccessList{}
				for _, access := range a {
					if access.Namespace == accesscontrol.All {
						result = append(result, access)
					}
				}
				a = result
			}
			if len(a) > 0 {
				verbAccess[verb] = a
			}
		}

		if len(verbAccess) == 0 {
			if gr.Group == "" && gr.Resource == "namespaces" {
				var accessList accesscontrol.AccessList
				for _, ns := range access.Namespaces() {
					accessList = append(accessList, accesscontrol.Access{
						Namespace:    accesscontrol.All,
						ResourceName: ns,
					})
				}
				verbAccess["list"] = accessList
				verbAccess["watch"] = accessList
			} else {
				continue
			}
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

		if err := result.AddSchema(*s); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (c *Collection) applyTemplates(schema *types.APISchema) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	templates := []*Template{
		c.templates[schema.ID],
		c.templates[fmt.Sprintf("%s/%s", attributes.Group(schema), attributes.Kind(schema))],
		c.templates[""],
	}

	for _, t := range templates {
		if t == nil {
			continue
		}
		if schema.Formatter == nil {
			schema.Formatter = t.Formatter
		} else if t.Formatter != nil {
			schema.Formatter = types.FormatterChain(t.Formatter, schema.Formatter)
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
	}
}
