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

package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/internal/log"
)

var log = logf.RuntimeLog.WithName("predicate").WithName("eventFilters")

// Predicate filters events before enqueuing the keys.
type Predicate interface {
	// Create returns true if the Create event should be processed
	Create(event.CreateEvent) bool

	// Delete returns true if the Delete event should be processed
	Delete(event.DeleteEvent) bool

	// Update returns true if the Update event should be processed
	Update(event.UpdateEvent) bool

	// Generic returns true if the Generic event should be processed
	Generic(event.GenericEvent) bool
}

var _ Predicate = Funcs{}
var _ Predicate = ResourceVersionChangedPredicate{}
var _ Predicate = GenerationChangedPredicate{}

// Funcs is a function that implements Predicate.
type Funcs struct {
	// Create returns true if the Create event should be processed
	CreateFunc func(event.CreateEvent) bool

	// Delete returns true if the Delete event should be processed
	DeleteFunc func(event.DeleteEvent) bool

	// Update returns true if the Update event should be processed
	UpdateFunc func(event.UpdateEvent) bool

	// Generic returns true if the Generic event should be processed
	GenericFunc func(event.GenericEvent) bool
}

// Create implements Predicate
func (p Funcs) Create(e event.CreateEvent) bool {
	if p.CreateFunc != nil {
		return p.CreateFunc(e)
	}
	return true
}

// Delete implements Predicate
func (p Funcs) Delete(e event.DeleteEvent) bool {
	if p.DeleteFunc != nil {
		return p.DeleteFunc(e)
	}
	return true
}

// Update implements Predicate
func (p Funcs) Update(e event.UpdateEvent) bool {
	if p.UpdateFunc != nil {
		return p.UpdateFunc(e)
	}
	return true
}

// Generic implements Predicate
func (p Funcs) Generic(e event.GenericEvent) bool {
	if p.GenericFunc != nil {
		return p.GenericFunc(e)
	}
	return true
}

// ResourceVersionChangedPredicate implements a default update predicate function on resource version change
type ResourceVersionChangedPredicate struct {
	Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (ResourceVersionChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.MetaOld == nil {
		log.Error(nil, "UpdateEvent has no old metadata", "event", e)
		return false
	}
	if e.ObjectOld == nil {
		log.Error(nil, "GenericEvent has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "GenericEvent has no new runtime object for update", "event", e)
		return false
	}
	if e.MetaNew == nil {
		log.Error(nil, "UpdateEvent has no new metadata", "event", e)
		return false
	}
	return e.MetaNew.GetResourceVersion() != e.MetaOld.GetResourceVersion()
}

// GenerationChangedPredicate implements a default update predicate function on Generation change.
//
// This predicate will skip update events that have no change in the object's metadata.generation field.
// The metadata.generation field of an object is incremented by the API server when writes are made to the spec field of an object.
// This allows a controller to ignore update events where the spec is unchanged, and only the metadata and/or status fields are changed.
//
// For CustomResource objects the Generation is only incremented when the status subresource is enabled.
//
// Caveats:
//
// * The assumption that the Generation is incremented only on writing to the spec does not hold for all APIs.
// E.g For Deployment objects the Generation is also incremented on writes to the metadata.annotations field.
// For object types other than CustomResources be sure to verify which fields will trigger a Generation increment when they are written to.
//
// * With this predicate, any update events with writes only to the status field will not be reconciled.
// So in the event that the status block is overwritten or wiped by someone else the controller will not self-correct to restore the correct status.
type GenerationChangedPredicate struct {
	Funcs
}

// Update implements default UpdateEvent filter for validating generation change
func (GenerationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.MetaOld == nil {
		log.Error(nil, "Update event has no old metadata", "event", e)
		return false
	}
	if e.ObjectOld == nil {
		log.Error(nil, "Update event has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "Update event has no new runtime object for update", "event", e)
		return false
	}
	if e.MetaNew == nil {
		log.Error(nil, "Update event has no new metadata", "event", e)
		return false
	}
	return e.MetaNew.GetGeneration() != e.MetaOld.GetGeneration()
}
