package schema

import (
	"context"
	"strings"
	"sync"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/cache"
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
	cache      *cache.LRUExpireCache
	lock       sync.RWMutex

	ctx     context.Context
	running map[string]func()
	as      accesscontrol.AccessSetLookup
}

type Template struct {
	Group        string
	Kind         string
	ID           string
	Customize    func(*types.APISchema)
	Formatter    types.Formatter
	Store        types.Store
	Start        func(ctx context.Context) error
	StoreFactory func(types.Store) types.Store
}

func NewCollection(ctx context.Context, baseSchema *types.APISchemas, access accesscontrol.AccessSetLookup) *Collection {
	return &Collection{
		baseSchema: baseSchema,
		schemas:    map[string]*types.APISchema{},
		templates:  map[string]*Template{},
		byGVR:      map[schema.GroupVersionResource]string{},
		byGVK:      map[schema.GroupVersionKind]string{},
		cache:      cache.NewLRUExpireCache(1000),
		ctx:        ctx,
		as:         access,
		running:    map[string]func(){},
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

		c.applyTemplates(s)
	}

	c.lock.Lock()
	c.startStopTemplate(schemas)
	c.schemas = schemas
	c.byGVR = byGVR
	c.byGVK = byGVK
	for _, k := range c.cache.Keys() {
		c.cache.Remove(k)
	}
	c.lock.Unlock()
}

func (c *Collection) startStopTemplate(schemas map[string]*types.APISchema) {
	for id := range schemas {
		if _, ok := c.running[id]; ok {
			continue
		}
		template := c.templates[id]
		if template == nil || template.Start == nil {
			continue
		}

		subCtx, cancel := context.WithCancel(c.ctx)
		if err := template.Start(subCtx); err != nil {
			logrus.Errorf("failed to start schema template: %s", id)
			continue
		}
		c.running[id] = cancel
	}

	for id, cancel := range c.running {
		if _, ok := schemas[id]; !ok {
			cancel()
			delete(c.running, id)
		}
	}
}

func (c *Collection) Schema(id string) *types.APISchema {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.schemas[id]
}

func (c *Collection) IDs() (result []string) {
	c.lock.RLock()
	defer c.lock.RUnlock()

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
	c.lock.RLock()
	defer c.lock.RUnlock()

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
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.byGVK[gvk]
}

func (c *Collection) AddTemplate(template *Template) {
	c.lock.RLock()
	defer c.lock.RUnlock()

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
