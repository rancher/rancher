package rbac

import (
	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/types"
)

type contextKey struct{}

type contextBased struct {
	all    authorization.AllAccess
	lookup contextLookup
}

type contextLookup func(ctx *types.APIContext) (types.AccessControl, bool)

func newContextBased(lookup contextLookup) types.AccessControl {
	return &contextBased{
		lookup: lookup,
	}
}

func (c *contextBased) Expire(apiContext *types.APIContext, schema *types.Schema) {
	ac, ok := c.lookup(apiContext)
	if !ok {
		return
	}
	if e, ok := ac.(types.Expire); ok {
		e.Expire(apiContext, schema)
	}
}

func (c *contextBased) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.CanCreate(apiContext, schema)
	}
	return c.all.CanCreate(apiContext, schema)
}

func (c *contextBased) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.CanList(apiContext, schema)
	}
	return c.all.CanList(apiContext, schema)
}

func (c *contextBased) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.CanGet(apiContext, schema)
	}
	return c.all.CanGet(apiContext, schema)
}

func (c *contextBased) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.CanUpdate(apiContext, obj, schema)
	}
	return c.all.CanUpdate(apiContext, obj, schema)
}

func (c *contextBased) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.CanDelete(apiContext, obj, schema)
	}
	return c.all.CanDelete(apiContext, obj, schema)
}

func (c *contextBased) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.CanDo(apiGroup, resource, verb, apiContext, obj, schema)
	}
	return c.all.CanDo(apiGroup, resource, verb, apiContext, obj, schema)
}

func (c *contextBased) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.Filter(apiContext, schema, obj, context)
	}
	return obj
}

func (c *contextBased) FilterList(apiContext *types.APIContext, schema *types.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	ac, ok := c.lookup(apiContext)
	if ok {
		return ac.FilterList(apiContext, schema, obj, context)
	}
	return obj
}
