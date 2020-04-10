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

package builder

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Supporting mocking out functions for testing
var newController = controller.New
var getGvk = apiutil.GVKForObject

// Builder builds a Controller.
type Builder struct {
	forInput         ForInput
	ownsInput        []OwnsInput
	watchesInput     []WatchesInput
	mgr              manager.Manager
	globalPredicates []predicate.Predicate
	config           *rest.Config
	ctrl             controller.Controller
	ctrlOptions      controller.Options
	name             string
}

// ControllerManagedBy returns a new controller builder that will be started by the provided Manager
func ControllerManagedBy(m manager.Manager) *Builder {
	return &Builder{mgr: m}
}

// ForType defines the type of Object being *reconciled*, and configures the ControllerManagedBy to respond to create / delete /
// update events by *reconciling the object*.
// This is the equivalent of calling
// Watches(&source.Kind{Type: apiType}, &handler.EnqueueRequestForObject{})
//
// Deprecated: Use For
func (blder *Builder) ForType(apiType runtime.Object) *Builder {
	return blder.For(apiType)
}

// ForInput represents the information set by For method.
type ForInput struct {
	object     runtime.Object
	predicates []predicate.Predicate
}

// For defines the type of Object being *reconciled*, and configures the ControllerManagedBy to respond to create / delete /
// update events by *reconciling the object*.
// This is the equivalent of calling
// Watches(&source.Kind{Type: apiType}, &handler.EnqueueRequestForObject{})
func (blder *Builder) For(object runtime.Object, opts ...ForOption) *Builder {
	input := ForInput{object: object}
	for _, opt := range opts {
		opt.ApplyToFor(&input)
	}

	blder.forInput = input
	return blder
}

// OwnsInput represents the information set by Owns method.
type OwnsInput struct {
	object     runtime.Object
	predicates []predicate.Predicate
}

// Owns defines types of Objects being *generated* by the ControllerManagedBy, and configures the ControllerManagedBy to respond to
// create / delete / update events by *reconciling the owner object*.  This is the equivalent of calling
// Watches(&source.Kind{Type: <ForType-forInput>}, &handler.EnqueueRequestForOwner{OwnerType: apiType, IsController: true})
func (blder *Builder) Owns(object runtime.Object, opts ...OwnsOption) *Builder {
	input := OwnsInput{object: object}
	for _, opt := range opts {
		opt.ApplyToOwns(&input)
	}

	blder.ownsInput = append(blder.ownsInput, input)
	return blder
}

// WatchesInput represents the information set by Watches method.
type WatchesInput struct {
	src          source.Source
	eventhandler handler.EventHandler
	predicates   []predicate.Predicate
}

// Watches exposes the lower-level ControllerManagedBy Watches functions through the builder.  Consider using
// Owns or For instead of Watches directly.
// Specified predicates are registered only for given source.
func (blder *Builder) Watches(src source.Source, eventhandler handler.EventHandler, opts ...WatchesOption) *Builder {
	input := WatchesInput{src: src, eventhandler: eventhandler}
	for _, opt := range opts {
		opt.ApplyToWatches(&input)
	}

	blder.watchesInput = append(blder.watchesInput, input)
	return blder
}

// WithConfig sets the Config to use for configuring clients.  Defaults to the in-cluster config or to ~/.kube/config.
//
// Deprecated: Use ControllerManagedBy(Manager) and this isn't needed.
func (blder *Builder) WithConfig(config *rest.Config) *Builder {
	blder.config = config
	return blder
}

// WithEventFilter sets the event filters, to filter which create/update/delete/generic events eventually
// trigger reconciliations.  For example, filtering on whether the resource version has changed.
// Given predicate is added for all watched objects.
// Defaults to the empty list.
func (blder *Builder) WithEventFilter(p predicate.Predicate) *Builder {
	blder.globalPredicates = append(blder.globalPredicates, p)
	return blder
}

// WithOptions overrides the controller options use in doController. Defaults to empty.
func (blder *Builder) WithOptions(options controller.Options) *Builder {
	blder.ctrlOptions = options
	return blder
}

// Named sets the name of the controller to the given name.  The name shows up
// in metrics, among other things, and thus should be a prometheus compatible name
// (underscores and alphanumeric characters only).
//
// By default, controllers are named using the lowercase version of their kind.
func (blder *Builder) Named(name string) *Builder {
	blder.name = name
	return blder
}

// Complete builds the Application ControllerManagedBy.
func (blder *Builder) Complete(r reconcile.Reconciler) error {
	_, err := blder.Build(r)
	return err
}

// Build builds the Application ControllerManagedBy and returns the Controller it created.
func (blder *Builder) Build(r reconcile.Reconciler) (controller.Controller, error) {
	if r == nil {
		return nil, fmt.Errorf("must provide a non-nil Reconciler")
	}
	if blder.mgr == nil {
		return nil, fmt.Errorf("must provide a non-nil Manager")
	}

	// Set the Config
	blder.loadRestConfig()

	// Set the ControllerManagedBy
	if err := blder.doController(r); err != nil {
		return nil, err
	}

	// Set the Watch
	if err := blder.doWatch(); err != nil {
		return nil, err
	}

	return blder.ctrl, nil
}

func (blder *Builder) doWatch() error {
	// Reconcile type
	src := &source.Kind{Type: blder.forInput.object}
	hdler := &handler.EnqueueRequestForObject{}
	allPredicates := append(blder.globalPredicates, blder.forInput.predicates...)
	err := blder.ctrl.Watch(src, hdler, allPredicates...)
	if err != nil {
		return err
	}

	// Watches the managed types
	for _, own := range blder.ownsInput {
		src := &source.Kind{Type: own.object}
		hdler := &handler.EnqueueRequestForOwner{
			OwnerType:    blder.forInput.object,
			IsController: true,
		}
		allPredicates := append([]predicate.Predicate(nil), blder.globalPredicates...)
		allPredicates = append(allPredicates, own.predicates...)
		if err := blder.ctrl.Watch(src, hdler, allPredicates...); err != nil {
			return err
		}
	}

	// Do the watch requests
	for _, w := range blder.watchesInput {
		allPredicates := append([]predicate.Predicate(nil), blder.globalPredicates...)
		allPredicates = append(allPredicates, w.predicates...)
		if err := blder.ctrl.Watch(w.src, w.eventhandler, allPredicates...); err != nil {
			return err
		}

	}
	return nil
}

func (blder *Builder) loadRestConfig() {
	if blder.config == nil {
		blder.config = blder.mgr.GetConfig()
	}
}

func (blder *Builder) getControllerName() (string, error) {
	if blder.name != "" {
		return blder.name, nil
	}
	gvk, err := getGvk(blder.forInput.object, blder.mgr.GetScheme())
	if err != nil {
		return "", err
	}
	return strings.ToLower(gvk.Kind), nil
}

func (blder *Builder) doController(r reconcile.Reconciler) error {
	name, err := blder.getControllerName()
	if err != nil {
		return err
	}
	ctrlOptions := blder.ctrlOptions
	ctrlOptions.Reconciler = r
	blder.ctrl, err = newController(name, blder.mgr, ctrlOptions)
	return err
}
