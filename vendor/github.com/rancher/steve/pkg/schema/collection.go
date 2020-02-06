package schema

import (
	"context"
	"strings"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/table"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/schemas"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
)

type Factory interface {
	Schemas(user user.Info) (*types.APISchemas, error)
	ByGVR(gvr schema.GroupVersionResource) string
	ByGVK(gvr schema.GroupVersionKind) string
}

type Collection struct {
	toSync     int32
	baseSchema *types.APISchemas
	schemas    map[string]*types.APISchema
	templates  map[string]*Template
	byGVR      map[schema.GroupVersionResource]string
	byGVK      map[schema.GroupVersionKind]string

	as accesscontrol.AccessSetLookup
}

type Template struct {
	Group           string
	Kind            string
	ID              string
	Customize       func(*types.APISchema)
	Formatter       types.Formatter
	Store           types.Store
	Start           func(ctx context.Context) error
	StoreFactory    func(types.Store) types.Store
	Mapper          schemas.Mapper
	Columns         []table.Column
	ComputedColumns func(data.Object)
}

func NewCollection(baseSchema *types.APISchemas, access accesscontrol.AccessSetLookup) *Collection {
	return &Collection{
		baseSchema: baseSchema,
		schemas:    map[string]*types.APISchema{},
		templates:  map[string]*Template{},
		byGVR:      map[schema.GroupVersionResource]string{},
		byGVK:      map[schema.GroupVersionKind]string{},
		as:         access,
	}
}

func (c *Collection) Reset(schemas map[string]*types.APISchema) {
	byGVK := map[schema.GroupVersionKind]string{}
	byGVR := map[schema.GroupVersionResource]string{}

	for _, s := range schemas {
		gvr := attributes.GVR(s)
		if gvr.Resource != "" {
			byGVR[gvr] = s.ID
		}
		gvk := attributes.GVK(s)
		if gvk.Kind != "" {
			byGVK[gvk] = s.ID
		}
	}

	c.schemas = schemas
	c.byGVR = byGVR
	c.byGVK = byGVK
}

func (c *Collection) Schema(id string) *types.APISchema {
	return c.schemas[id]
}

func (c *Collection) IDs() (result []string) {
	seen := map[string]bool{}
	for _, id := range c.byGVR {
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return
}

func (c *Collection) ByGVR(gvr schema.GroupVersionResource) string {
	id, ok := c.byGVR[gvr]
	if ok {
		return id
	}
	gvr.Resource = name.GuessPluralName(strings.ToLower(gvr.Resource))
	return c.byGVK[schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    gvr.Resource,
	}]
}

func (c *Collection) ByGVK(gvk schema.GroupVersionKind) string {
	return c.byGVK[gvk]
}

func (c *Collection) TemplateForSchemaID(id string) *Template {
	return c.templates[id]
}

func (c *Collection) AddTemplate(template *Template) {
	if template.Kind != "" {
		c.templates[template.Group+"/"+template.Kind] = template
	}
	if template.ID != "" {
		c.templates[template.ID] = template
	}
	if template.Kind == "" && template.Group == "" && template.ID == "" {
		c.templates[""] = template
	}
}
