package lifecycle

import (
	"fmt"
	"reflect"

	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types/slice"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	created            = "lifecycle.cattle.io/create"
	finalizerKey       = "controller.cattle.io/"
	ScopedFinalizerKey = "clusterscoped.controller.cattle.io/"
)

type ObjectLifecycle interface {
	Create(obj runtime.Object) (runtime.Object, error)
	Finalize(obj runtime.Object) (runtime.Object, error)
	Updated(obj runtime.Object) (runtime.Object, error)
}

type objectLifecycleAdapter struct {
	name          string
	clusterScoped bool
	lifecycle     ObjectLifecycle
	objectClient  *objectclient.ObjectClient
}

func NewObjectLifecycleAdapter(name string, clusterScoped bool, lifecycle ObjectLifecycle, objectClient *objectclient.ObjectClient) func(key string, obj interface{}) (interface{}, error) {
	o := objectLifecycleAdapter{
		name:          name,
		clusterScoped: clusterScoped,
		lifecycle:     lifecycle,
		objectClient:  objectClient,
	}
	return o.sync
}

func (o *objectLifecycleAdapter) sync(key string, in interface{}) (interface{}, error) {
	if in == nil || reflect.ValueOf(in).IsNil() {
		return nil, nil
	}

	obj, ok := in.(runtime.Object)
	if !ok {
		return nil, nil
	}

	metadata, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	if newObj, cont, err := o.finalize(metadata, obj); err != nil || !cont {
		return nil, err
	} else if newObj != nil {
		obj = newObj
	}

	if newObj, cont, err := o.create(metadata, obj); err != nil || !cont {
		return nil, err
	} else if newObj != nil {
		obj = newObj
	}

	copyObj := obj.DeepCopyObject()
	newObj, err := o.lifecycle.Updated(copyObj)
	if newObj != nil {
		return o.update(metadata.GetName(), obj, newObj)
	}
	return nil, err
}

func (o *objectLifecycleAdapter) update(name string, orig, obj runtime.Object) (runtime.Object, error) {
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		return o.objectClient.Update(name, obj)
	}
	return obj, nil
}

func (o *objectLifecycleAdapter) finalize(metadata metav1.Object, obj runtime.Object) (runtime.Object, bool, error) {
	// Check finalize
	if metadata.GetDeletionTimestamp() == nil {
		return nil, true, nil
	}

	if !slice.ContainsString(metadata.GetFinalizers(), o.constructFinalizerKey()) {
		return nil, false, nil
	}

	copyObj := obj.DeepCopyObject()
	if newObj, err := o.lifecycle.Finalize(copyObj); err != nil {
		if newObj != nil {
			newObj, _ := o.update(metadata.GetName(), obj, newObj)
			return newObj, false, err
		}
		return nil, false, err
	} else if newObj != nil {
		copyObj = newObj
	}

	newObj, err := o.removeFinalizer(o.constructFinalizerKey(), copyObj)
	return newObj, false, err
}

func (o *objectLifecycleAdapter) removeFinalizer(name string, obj runtime.Object) (runtime.Object, error) {
	for i := 0; i < 3; i++ {
		metadata, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}

		var finalizers []string
		for _, finalizer := range metadata.GetFinalizers() {
			if finalizer == name {
				continue
			}
			finalizers = append(finalizers, finalizer)
		}
		metadata.SetFinalizers(finalizers)

		newObj, err := o.objectClient.Update(metadata.GetName(), obj)
		if err == nil {
			return newObj, nil
		}

		obj, err = o.objectClient.GetNamespaced(metadata.GetNamespace(), metadata.GetName(), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("failed to remove finalizer on %s", name)
}

func (o *objectLifecycleAdapter) createKey() string {
	return created + "." + o.name
}

func (o *objectLifecycleAdapter) constructFinalizerKey() string {
	if o.clusterScoped {
		return ScopedFinalizerKey + o.name
	}
	return finalizerKey + o.name
}

func (o *objectLifecycleAdapter) create(metadata metav1.Object, obj runtime.Object) (runtime.Object, bool, error) {
	if o.isInitialized(metadata) {
		return nil, true, nil
	}

	copyObj := obj.DeepCopyObject()
	copyObj, err := o.addFinalizer(copyObj)
	if err != nil {
		return copyObj, false, err
	}

	if newObj, err := o.lifecycle.Create(copyObj); err != nil {
		newObj, _ = o.update(metadata.GetName(), obj, newObj)
		return newObj, false, err
	} else if newObj != nil {
		copyObj = newObj
	}

	newObj, err := o.setInitialized(copyObj)
	return newObj, false, err
}

func (o *objectLifecycleAdapter) isInitialized(metadata metav1.Object) bool {
	initialized := o.createKey()
	return metadata.GetAnnotations()[initialized] == "true"
}

func (o *objectLifecycleAdapter) setInitialized(obj runtime.Object) (runtime.Object, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	initialized := o.createKey()

	if metadata.GetAnnotations() == nil {
		metadata.SetAnnotations(map[string]string{})
	}
	metadata.GetAnnotations()[initialized] = "true"

	return o.objectClient.Update(metadata.GetName(), obj)
}

func (o *objectLifecycleAdapter) addFinalizer(obj runtime.Object) (runtime.Object, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	if slice.ContainsString(metadata.GetFinalizers(), o.constructFinalizerKey()) {
		return obj, nil
	}

	metadata.SetFinalizers(append(metadata.GetFinalizers(), o.constructFinalizerKey()))
	return o.objectClient.Update(metadata.GetName(), obj)
}
