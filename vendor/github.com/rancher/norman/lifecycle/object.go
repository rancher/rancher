package lifecycle

import (
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types/slice"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	created      = "lifecycle.cattle.io/create"
	finalizerKey = "controller.cattle.io/"
)

type ObjectLifecycle interface {
	Create(obj runtime.Object) (runtime.Object, error)
	Finalize(obj runtime.Object) (runtime.Object, error)
	Updated(obj runtime.Object) (runtime.Object, error)
}

type objectLifecycleAdapter struct {
	name         string
	lifecycle    ObjectLifecycle
	objectClient *clientbase.ObjectClient
}

func NewObjectLifecycleAdapter(name string, lifecycle ObjectLifecycle, objectClient *clientbase.ObjectClient) func(key string, obj runtime.Object) error {
	o := objectLifecycleAdapter{
		name:         name,
		lifecycle:    lifecycle,
		objectClient: objectClient,
	}
	return o.sync
}

func (o *objectLifecycleAdapter) sync(key string, obj runtime.Object) error {
	if obj == nil {
		return nil
	}

	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if cont, err := o.finalize(metadata, obj); err != nil || !cont {
		return err
	}

	if cont, err := o.create(metadata, obj); err != nil || !cont {
		return err
	}

	obj = obj.DeepCopyObject()
	if newObj, err := o.lifecycle.Updated(obj); err != nil {
		if newObj != nil {
			o.objectClient.Update(metadata.GetName(), newObj)
		}
		return err
	} else if newObj != nil {
		_, err = o.objectClient.Update(metadata.GetName(), newObj)
		return err
	}

	return nil
}

func (o *objectLifecycleAdapter) finalize(metadata metav1.Object, obj runtime.Object) (bool, error) {
	// Check finalize
	if metadata.GetDeletionTimestamp() == nil {
		return true, nil
	}

	if !slice.ContainsString(metadata.GetFinalizers(), o.constructFinalizerKey()) {
		return false, nil
	}

	obj = obj.DeepCopyObject()
	if newObj, err := o.lifecycle.Finalize(obj); err != nil {
		if newObj != nil {
			o.objectClient.Update(metadata.GetName(), newObj)
		}
		return false, err
	} else if newObj != nil {
		obj = newObj
	}

	if err := removeFinalizer(o.constructFinalizerKey(), obj); err != nil {
		return false, err
	}

	_, err := o.objectClient.Update(metadata.GetName(), obj)
	return false, err
}

func removeFinalizer(name string, obj runtime.Object) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	var finalizers []string
	for _, finalizer := range metadata.GetFinalizers() {
		if finalizer == name {
			continue
		}
		finalizers = append(finalizers, finalizer)
	}
	metadata.SetFinalizers(finalizers)

	return nil
}

func (o *objectLifecycleAdapter) createKey() string {
	return created + "." + o.name
}

func (o *objectLifecycleAdapter) constructFinalizerKey() string {
	return finalizerKey + o.name
}

func (o *objectLifecycleAdapter) create(metadata metav1.Object, obj runtime.Object) (bool, error) {
	initialized := o.createKey()

	if metadata.GetAnnotations()[initialized] == "true" {
		return true, nil
	}

	obj = obj.DeepCopyObject()
	if newObj, err := o.lifecycle.Create(obj); err != nil {
		if newObj != nil {
			o.objectClient.Update(metadata.GetName(), newObj)
		}
		return false, err
	} else if newObj != nil {
		obj = newObj
	}

	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	if metadata.GetAnnotations() == nil {
		metadata.SetAnnotations(map[string]string{})
	}

	if o.objectClient.GroupVersionKind().Kind != "Namespace" {
		metadata.SetFinalizers(append(metadata.GetFinalizers(), o.constructFinalizerKey()))
	}
	metadata.GetAnnotations()[initialized] = "true"

	_, err = o.objectClient.Update(metadata.GetName(), obj)
	return false, err
}
