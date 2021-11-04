/*
Copyright 2021 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v3

import (
	"context"
	"time"

	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type CatalogTemplateHandler func(string, *v3.CatalogTemplate) (*v3.CatalogTemplate, error)

type CatalogTemplateController interface {
	generic.ControllerMeta
	CatalogTemplateClient

	OnChange(ctx context.Context, name string, sync CatalogTemplateHandler)
	OnRemove(ctx context.Context, name string, sync CatalogTemplateHandler)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, duration time.Duration)

	Cache() CatalogTemplateCache
}

type CatalogTemplateClient interface {
	Create(*v3.CatalogTemplate) (*v3.CatalogTemplate, error)
	Update(*v3.CatalogTemplate) (*v3.CatalogTemplate, error)
	UpdateStatus(*v3.CatalogTemplate) (*v3.CatalogTemplate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	Get(namespace, name string, options metav1.GetOptions) (*v3.CatalogTemplate, error)
	List(namespace string, opts metav1.ListOptions) (*v3.CatalogTemplateList, error)
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.CatalogTemplate, err error)
}

type CatalogTemplateCache interface {
	Get(namespace, name string) (*v3.CatalogTemplate, error)
	List(namespace string, selector labels.Selector) ([]*v3.CatalogTemplate, error)

	AddIndexer(indexName string, indexer CatalogTemplateIndexer)
	GetByIndex(indexName, key string) ([]*v3.CatalogTemplate, error)
}

type CatalogTemplateIndexer func(obj *v3.CatalogTemplate) ([]string, error)

type catalogTemplateController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewCatalogTemplateController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) CatalogTemplateController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &catalogTemplateController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromCatalogTemplateHandlerToHandler(sync CatalogTemplateHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v3.CatalogTemplate
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v3.CatalogTemplate))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *catalogTemplateController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v3.CatalogTemplate))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateCatalogTemplateDeepCopyOnChange(client CatalogTemplateClient, obj *v3.CatalogTemplate, handler func(obj *v3.CatalogTemplate) (*v3.CatalogTemplate, error)) (*v3.CatalogTemplate, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *catalogTemplateController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *catalogTemplateController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *catalogTemplateController) OnChange(ctx context.Context, name string, sync CatalogTemplateHandler) {
	c.AddGenericHandler(ctx, name, FromCatalogTemplateHandlerToHandler(sync))
}

func (c *catalogTemplateController) OnRemove(ctx context.Context, name string, sync CatalogTemplateHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromCatalogTemplateHandlerToHandler(sync)))
}

func (c *catalogTemplateController) Enqueue(namespace, name string) {
	c.controller.Enqueue(namespace, name)
}

func (c *catalogTemplateController) EnqueueAfter(namespace, name string, duration time.Duration) {
	c.controller.EnqueueAfter(namespace, name, duration)
}

func (c *catalogTemplateController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *catalogTemplateController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *catalogTemplateController) Cache() CatalogTemplateCache {
	return &catalogTemplateCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *catalogTemplateController) Create(obj *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	result := &v3.CatalogTemplate{}
	return result, c.client.Create(context.TODO(), obj.Namespace, obj, result, metav1.CreateOptions{})
}

func (c *catalogTemplateController) Update(obj *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	result := &v3.CatalogTemplate{}
	return result, c.client.Update(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *catalogTemplateController) UpdateStatus(obj *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	result := &v3.CatalogTemplate{}
	return result, c.client.UpdateStatus(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *catalogTemplateController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), namespace, name, *options)
}

func (c *catalogTemplateController) Get(namespace, name string, options metav1.GetOptions) (*v3.CatalogTemplate, error) {
	result := &v3.CatalogTemplate{}
	return result, c.client.Get(context.TODO(), namespace, name, result, options)
}

func (c *catalogTemplateController) List(namespace string, opts metav1.ListOptions) (*v3.CatalogTemplateList, error) {
	result := &v3.CatalogTemplateList{}
	return result, c.client.List(context.TODO(), namespace, result, opts)
}

func (c *catalogTemplateController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), namespace, opts)
}

func (c *catalogTemplateController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (*v3.CatalogTemplate, error) {
	result := &v3.CatalogTemplate{}
	return result, c.client.Patch(context.TODO(), namespace, name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type catalogTemplateCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *catalogTemplateCache) Get(namespace, name string) (*v3.CatalogTemplate, error) {
	obj, exists, err := c.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v3.CatalogTemplate), nil
}

func (c *catalogTemplateCache) List(namespace string, selector labels.Selector) (ret []*v3.CatalogTemplate, err error) {

	err = cache.ListAllByNamespace(c.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v3.CatalogTemplate))
	})

	return ret, err
}

func (c *catalogTemplateCache) AddIndexer(indexName string, indexer CatalogTemplateIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v3.CatalogTemplate))
		},
	}))
}

func (c *catalogTemplateCache) GetByIndex(indexName, key string) (result []*v3.CatalogTemplate, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v3.CatalogTemplate, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v3.CatalogTemplate))
	}
	return result, nil
}

type CatalogTemplateStatusHandler func(obj *v3.CatalogTemplate, status v3.TemplateStatus) (v3.TemplateStatus, error)

type CatalogTemplateGeneratingHandler func(obj *v3.CatalogTemplate, status v3.TemplateStatus) ([]runtime.Object, v3.TemplateStatus, error)

func RegisterCatalogTemplateStatusHandler(ctx context.Context, controller CatalogTemplateController, condition condition.Cond, name string, handler CatalogTemplateStatusHandler) {
	statusHandler := &catalogTemplateStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromCatalogTemplateHandlerToHandler(statusHandler.sync))
}

func RegisterCatalogTemplateGeneratingHandler(ctx context.Context, controller CatalogTemplateController, apply apply.Apply,
	condition condition.Cond, name string, handler CatalogTemplateGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &catalogTemplateGeneratingHandler{
		CatalogTemplateGeneratingHandler: handler,
		apply:                            apply,
		name:                             name,
		gvk:                              controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterCatalogTemplateStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type catalogTemplateStatusHandler struct {
	client    CatalogTemplateClient
	condition condition.Cond
	handler   CatalogTemplateStatusHandler
}

func (a *catalogTemplateStatusHandler) sync(key string, obj *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type catalogTemplateGeneratingHandler struct {
	CatalogTemplateGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *catalogTemplateGeneratingHandler) Remove(key string, obj *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v3.CatalogTemplate{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *catalogTemplateGeneratingHandler) Handle(obj *v3.CatalogTemplate, status v3.TemplateStatus) (v3.TemplateStatus, error) {
	objs, newStatus, err := a.CatalogTemplateGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
