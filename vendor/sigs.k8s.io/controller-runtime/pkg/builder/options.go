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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// {{{ "Functional" Option Interfaces

// ForOption is some configuration that modifies options for a For request.
type ForOption interface {
	// ApplyToFor applies this configuration to the given for input.
	ApplyToFor(*ForInput)
}

// OwnsOption is some configuration that modifies options for a owns request.
type OwnsOption interface {
	// ApplyToOwns applies this configuration to the given owns input.
	ApplyToOwns(*OwnsInput)
}

// WatchesOption is some configuration that modifies options for a watches request.
type WatchesOption interface {
	// ApplyToWatches applies this configuration to the given watches options.
	ApplyToWatches(*WatchesInput)
}

// }}}

// {{{ Multi-Type Options

// WithPredicates sets the given predicates list.
func WithPredicates(predicates ...predicate.Predicate) Predicates {
	return Predicates{
		predicates: predicates,
	}
}

// Predicates filters events before enqueuing the keys.
type Predicates struct {
	predicates []predicate.Predicate
}

// ApplyToFor applies this configuration to the given ForInput options.
func (w Predicates) ApplyToFor(opts *ForInput) {
	opts.predicates = w.predicates
}

// ApplyToOwns applies this configuration to the given OwnsInput options.
func (w Predicates) ApplyToOwns(opts *OwnsInput) {
	opts.predicates = w.predicates
}

// ApplyToWatches applies this configuration to the given WatchesInput options.
func (w Predicates) ApplyToWatches(opts *WatchesInput) {
	opts.predicates = w.predicates
}

var _ ForOption = &Predicates{}
var _ OwnsOption = &Predicates{}
var _ WatchesOption = &Predicates{}

// }}}
