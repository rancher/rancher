/*
Copyright 2018 The Kubernetes Authors.

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

package controllerutil

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// AlreadyOwnedError is an error returned if the object you are trying to assign
// a controller reference is already owned by another controller Object is the
// subject and Owner is the reference for the current owner
type AlreadyOwnedError struct {
	Object metav1.Object
	Owner  metav1.OwnerReference
}

func (e *AlreadyOwnedError) Error() string {
	return fmt.Sprintf("Object %s/%s is already owned by another %s controller %s", e.Object.GetNamespace(), e.Object.GetName(), e.Owner.Kind, e.Owner.Name)
}

func newAlreadyOwnedError(Object metav1.Object, Owner metav1.OwnerReference) *AlreadyOwnedError {
	return &AlreadyOwnedError{
		Object: Object,
		Owner:  Owner,
	}
}

// SetControllerReference sets owner as a Controller OwnerReference on controlled.
// This is used for garbage collection of the controlled object and for
// reconciling the owner object on changes to controlled (with a Watch + EnqueueRequestForOwner).
// Since only one OwnerReference can be a controller, it returns an error if
// there is another OwnerReference with Controller flag set.
func SetControllerReference(owner, controlled metav1.Object, scheme *runtime.Scheme) error {
	// Validate the owner.
	ro, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("%T is not a runtime.Object, cannot call SetControllerReference", owner)
	}
	if err := validateOwner(owner, controlled); err != nil {
		return err
	}

	// Create a new controller ref.
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return err
	}
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(true),
		Controller:         pointer.BoolPtr(true),
	}

	// Return early with an error if the object is already controlled.
	if existing := metav1.GetControllerOf(controlled); existing != nil && !referSameObject(*existing, ref) {
		return newAlreadyOwnedError(controlled, *existing)
	}

	// Update owner references and return.
	upsertOwnerRef(ref, controlled)
	return nil
}

// SetOwnerReference is a helper method to make sure the given object contains an object reference to the object provided.
// This allows you to declare that owner has a dependency on the object without specifying it as a controller.
// If a reference to the same object already exists, it'll be overwritten with the newly provided version.
func SetOwnerReference(owner, object metav1.Object, scheme *runtime.Scheme) error {
	// Validate the owner.
	ro, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("%T is not a runtime.Object, cannot call SetOwnerReference", owner)
	}
	if err := validateOwner(owner, object); err != nil {
		return err
	}

	// Create a new owner ref.
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return err
	}
	ref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
	}

	// Update owner references and return.
	upsertOwnerRef(ref, object)
	return nil

}

func upsertOwnerRef(ref metav1.OwnerReference, object metav1.Object) {
	owners := object.GetOwnerReferences()
	idx := indexOwnerRef(owners, ref)
	if idx == -1 {
		owners = append(owners, ref)
	} else {
		owners[idx] = ref
	}
	object.SetOwnerReferences(owners)
}

// indexOwnerRef returns the index of the owner reference in the slice if found, or -1.
func indexOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) int {
	for index, r := range ownerReferences {
		if referSameObject(r, ref) {
			return index
		}
	}
	return -1
}

func validateOwner(owner, object metav1.Object) error {
	ownerNs := owner.GetNamespace()
	if ownerNs != "" {
		objNs := object.GetNamespace()
		if objNs == "" {
			return fmt.Errorf("cluster-scoped resource must not have a namespace-scoped owner, owner's namespace %s", ownerNs)
		}
		if ownerNs != objNs {
			return fmt.Errorf("cross-namespace owner references are disallowed, owner's namespace %s, obj's namespace %s", owner.GetNamespace(), object.GetNamespace())
		}
	}
	return nil
}

// Returns true if a and b point to the same object
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}

// OperationResult is the action result of a CreateOrUpdate call
type OperationResult string

const ( // They should complete the sentence "Deployment default/foo has been ..."
	// OperationResultNone means that the resource has not been changed
	OperationResultNone OperationResult = "unchanged"
	// OperationResultCreated means that a new resource is created
	OperationResultCreated OperationResult = "created"
	// OperationResultUpdated means that an existing resource is updated
	OperationResultUpdated OperationResult = "updated"
)

// CreateOrUpdate creates or updates the given object in the Kubernetes
// cluster. The object's desired state must be reconciled with the existing
// state inside the passed in callback MutateFn.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func CreateOrUpdate(ctx context.Context, c client.Client, obj runtime.Object, f MutateFn) (OperationResult, error) {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return OperationResultNone, err
	}

	if err := c.Get(ctx, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return OperationResultNone, err
		}
		if err := mutate(f, key, obj); err != nil {
			return OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return OperationResultNone, err
		}
		return OperationResultCreated, nil
	}

	existing := obj.DeepCopyObject()
	if err := mutate(f, key, obj); err != nil {
		return OperationResultNone, err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return OperationResultNone, nil
	}

	if err := c.Update(ctx, obj); err != nil {
		return OperationResultNone, err
	}
	return OperationResultUpdated, nil
}

// mutate wraps a MutateFn and applies validation to its result
func mutate(f MutateFn, key client.ObjectKey, obj runtime.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey, err := client.ObjectKeyFromObject(obj); err != nil || key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}

// MutateFn is a function which mutates the existing object into it's desired state.
type MutateFn func() error

// AddFinalizer accepts a metav1 object and adds the provided finalizer if not present.
func AddFinalizer(o metav1.Object, finalizer string) {
	f := o.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return
		}
	}
	o.SetFinalizers(append(f, finalizer))
}

// AddFinalizerWithError tries to convert a runtime object to a metav1 object and add the provided finalizer.
// It returns an error if the provided object cannot provide an accessor.
func AddFinalizerWithError(o runtime.Object, finalizer string) error {
	m, err := meta.Accessor(o)
	if err != nil {
		return err
	}
	AddFinalizer(m, finalizer)
	return nil
}

// RemoveFinalizer accepts a metav1 object and removes the provided finalizer if present.
func RemoveFinalizer(o metav1.Object, finalizer string) {
	f := o.GetFinalizers()
	for i, e := range f {
		if e == finalizer {
			f = append(f[:i], f[i+1:]...)
		}
	}
	o.SetFinalizers(f)
}

// RemoveFinalizerWithError tries to convert a runtime object to a metav1 object and remove the provided finalizer.
// It returns an error if the provided object cannot provide an accessor.
func RemoveFinalizerWithError(o runtime.Object, finalizer string) error {
	m, err := meta.Accessor(o)
	if err != nil {
		return err
	}
	RemoveFinalizer(m, finalizer)
	return nil
}
