/*
Copyright 2023 Rancher Labs, Inc.

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

package v1beta1

import (
	"context"
	"time"

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
	"k8s.io/apimachinery/pkg/watch"
	v1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// MachineController interface for managing Machine resources.
type MachineController interface {
	generic.ControllerMeta
	MachineClient

	// OnChange runs the given handler when the controller detects a resource was changed.
	OnChange(ctx context.Context, name string, sync MachineHandler)

	// OnRemove runs the given handler when the controller detects a resource was changed.
	OnRemove(ctx context.Context, name string, sync MachineHandler)

	// Enqueue adds the resource with the given name to the worker queue of the controller.
	Enqueue(namespace, name string)

	// EnqueueAfter runs Enqueue after the provided duration.
	EnqueueAfter(namespace, name string, duration time.Duration)

	// Cache returns a cache for the resource type T.
	Cache() MachineCache
}

// MachineClient interface for managing Machine resources in Kubernetes.
type MachineClient interface {
	// Create creates a new object and return the newly created Object or an error.
	Create(*v1beta1.Machine) (*v1beta1.Machine, error)

	// Update updates the object and return the newly updated Object or an error.
	Update(*v1beta1.Machine) (*v1beta1.Machine, error)
	// UpdateStatus updates the Status field of a the object and return the newly updated Object or an error.
	// Will always return an error if the object does not have a status field.
	UpdateStatus(*v1beta1.Machine) (*v1beta1.Machine, error)

	// Delete deletes the Object in the given name.
	Delete(namespace, name string, options *metav1.DeleteOptions) error

	// Get will attempt to retrieve the resource with the specified name.
	Get(namespace, name string, options metav1.GetOptions) (*v1beta1.Machine, error)

	// List will attempt to find multiple resources.
	List(namespace string, opts metav1.ListOptions) (*v1beta1.MachineList, error)

	// Watch will start watching resources.
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)

	// Patch will patch the resource with the matching name.
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.Machine, err error)
}

// MachineCache interface for retrieving Machine resources in memory.
type MachineCache interface {
	// Get returns the resources with the specified name from the cache.
	Get(namespace, name string) (*v1beta1.Machine, error)

	// List will attempt to find resources from the Cache.
	List(namespace string, selector labels.Selector) ([]*v1beta1.Machine, error)

	// AddIndexer adds  a new Indexer to the cache with the provided name.
	// If you call this after you already have data in the store, the results are undefined.
	AddIndexer(indexName string, indexer MachineIndexer)

	// GetByIndex returns the stored objects whose set of indexed values
	// for the named index includes the given indexed value.
	GetByIndex(indexName, key string) ([]*v1beta1.Machine, error)
}

// MachineHandler is function for performing any potential modifications to a Machine resource.
type MachineHandler func(string, *v1beta1.Machine) (*v1beta1.Machine, error)

// MachineIndexer computes a set of indexed values for the provided object.
type MachineIndexer func(obj *v1beta1.Machine) ([]string, error)

// MachineGenericController wraps wrangler/pkg/generic.Controller so that the function definitions adhere to MachineController interface.
type MachineGenericController struct {
	generic.ControllerInterface[*v1beta1.Machine, *v1beta1.MachineList]
}

// OnChange runs the given resource handler when the controller detects a resource was changed.
func (c *MachineGenericController) OnChange(ctx context.Context, name string, sync MachineHandler) {
	c.ControllerInterface.OnChange(ctx, name, generic.ObjectHandler[*v1beta1.Machine](sync))
}

// OnRemove runs the given object handler when the controller detects a resource was changed.
func (c *MachineGenericController) OnRemove(ctx context.Context, name string, sync MachineHandler) {
	c.ControllerInterface.OnRemove(ctx, name, generic.ObjectHandler[*v1beta1.Machine](sync))
}

// Cache returns a cache of resources in memory.
func (c *MachineGenericController) Cache() MachineCache {
	return &MachineGenericCache{
		c.ControllerInterface.Cache(),
	}
}

// MachineGenericCache wraps wrangler/pkg/generic.Cache so the function definitions adhere to MachineCache interface.
type MachineGenericCache struct {
	generic.CacheInterface[*v1beta1.Machine]
}

// AddIndexer adds  a new Indexer to the cache with the provided name.
// If you call this after you already have data in the store, the results are undefined.
func (c MachineGenericCache) AddIndexer(indexName string, indexer MachineIndexer) {
	c.CacheInterface.AddIndexer(indexName, generic.Indexer[*v1beta1.Machine](indexer))
}

type MachineStatusHandler func(obj *v1beta1.Machine, status v1beta1.MachineStatus) (v1beta1.MachineStatus, error)

type MachineGeneratingHandler func(obj *v1beta1.Machine, status v1beta1.MachineStatus) ([]runtime.Object, v1beta1.MachineStatus, error)

func FromMachineHandlerToHandler(sync MachineHandler) generic.Handler {
	return generic.FromObjectHandlerToHandler(generic.ObjectHandler[*v1beta1.Machine](sync))
}

func RegisterMachineStatusHandler(ctx context.Context, controller MachineController, condition condition.Cond, name string, handler MachineStatusHandler) {
	statusHandler := &machineStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromMachineHandlerToHandler(statusHandler.sync))
}

func RegisterMachineGeneratingHandler(ctx context.Context, controller MachineController, apply apply.Apply,
	condition condition.Cond, name string, handler MachineGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &machineGeneratingHandler{
		MachineGeneratingHandler: handler,
		apply:                    apply,
		name:                     name,
		gvk:                      controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterMachineStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type machineStatusHandler struct {
	client    MachineClient
	condition condition.Cond
	handler   MachineStatusHandler
}

func (a *machineStatusHandler) sync(key string, obj *v1beta1.Machine) (*v1beta1.Machine, error) {
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

type machineGeneratingHandler struct {
	MachineGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *machineGeneratingHandler) Remove(key string, obj *v1beta1.Machine) (*v1beta1.Machine, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1beta1.Machine{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *machineGeneratingHandler) Handle(obj *v1beta1.Machine, status v1beta1.MachineStatus) (v1beta1.MachineStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.MachineGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
