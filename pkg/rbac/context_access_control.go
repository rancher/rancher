package rbac

import (
	"errors"
	"sync"

	"github.com/rancher/norman/authorization"
	"github.com/rancher/norman/types"
)

var ErrNoContext = errors.New("no context found for access control")

type contextKey struct{}

type contextBased struct {
	all authorization.AllAccess
}

func NewContextBased() types.AccessControl {
	return &contextBased{}
}

func (c *contextBased) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.CanCreate(apiContext, schema)
	}
	return c.all.CanCreate(apiContext, schema)
}

func (c *contextBased) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.CanList(apiContext, schema)
	}
	return c.all.CanList(apiContext, schema)
}

func (c *contextBased) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.CanGet(apiContext, schema)
	}
	return c.all.CanGet(apiContext, schema)
}

func (c *contextBased) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.CanUpdate(apiContext, obj, schema)
	}
	return c.all.CanUpdate(apiContext, obj, schema)
}

func (c *contextBased) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.CanDelete(apiContext, obj, schema)
	}
	return c.all.CanDelete(apiContext, obj, schema)
}

func (c *contextBased) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.CanDo(apiGroup, resource, verb, apiContext, obj, schema)
	}
	return c.all.CanDo(apiGroup, resource, verb, apiContext, obj, schema)
}

func (c *contextBased) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.Filter(apiContext, schema, obj, context)
	}
	return obj
}

func (c *contextBased) FilterList(apiContext *types.APIContext, schema *types.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	ac, ok := apiContext.Request.Context().Value(contextKey{}).(types.AccessControl)
	if ok {
		return ac.FilterList(apiContext, schema, obj, context)
	}
	return obj
}

type accessControlFactory func(ctx *types.APIContext) types.AccessControl

type lazyContext struct {
	init    sync.Once
	factory accessControlFactory
	ac      types.AccessControl
}

func (l *lazyContext) get(apiContext *types.APIContext) types.AccessControl {
	l.init.Do(func() {
		l.ac = l.factory(apiContext)
	})
	return l.ac
}

func (l *lazyContext) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	return l.get(apiContext).CanCreate(apiContext, schema)
}

func (l *lazyContext) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	return l.get(apiContext).CanList(apiContext, schema)
}

func (l *lazyContext) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	return l.get(apiContext).CanGet(apiContext, schema)
}

func (l *lazyContext) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	return l.get(apiContext).CanUpdate(apiContext, obj, schema)
}

func (l *lazyContext) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	return l.get(apiContext).CanDelete(apiContext, obj, schema)
}

func (l *lazyContext) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	return l.get(apiContext).CanDo(apiGroup, resource, verb, apiContext, obj, schema)
}

func (l *lazyContext) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	return l.get(apiContext).Filter(apiContext, schema, obj, context)
}

func (l *lazyContext) FilterList(apiContext *types.APIContext, schema *types.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	return l.get(apiContext).FilterList(apiContext, schema, obj, context)
}
