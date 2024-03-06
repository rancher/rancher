package ensure

import (
	"errors"
	"fmt"

	"github.com/rancher/wrangler/pkg/generic"
)

// Ensurer is used to ensure that a group of objects has been created in k8s, and that all related corrupted versions
// of the object are properly processed.
type Ensurer[T generic.RuntimeMetaObject] struct {
	// ExistingStrategy is the strategy for retrieving existing objects that should be compared with the requested
	// objects
	ExistingStrategy ExistingStrategy[T]
	// ObjectIdentifier produces a string that is unique to T. Used to tell if a requested object
	// is covered by exising objects
	ObjectIdentifier func(T) string
	// IsValid is used to determine if a current object should be processed by ValidStrategy
	// or by InvalidStrategy
	IsValid func(obj T) bool
	// InvalidStrategy describes how to process invalid objects. Invalid objects will not be matched with requested objects
	// after processing, and are not altered further by Ensure.
	InvalidStrategy InvalidObjStrategy[T]
	// ValidStrategy describes how to process valid objects. Valid objects are objects which not currently covered by
	// an existing object, but were requested in parameters of Ensure
	ValidStrategy ValidObjStrategy[T]
}

// Ensure uses CurrentStrategy to determine what objects exist, IsValid to determine which objects are valid,
// InvalidStrategy to process the invalid objects, ObjectIdentifier to find out what objs are currently covered by valid
// objects, and ValidStrategy to process the the uncovered valid objects. Will return an error if it is not able to
// process any valid/invalid objects, but only after trying to process everything that can be confirmed.
func (e *Ensurer[T]) Ensure(objs ...T) error {
	current, err := e.ExistingStrategy.GetCurrent()
	if err != nil {
		return fmt.Errorf("unable to get current items: %w", err)
	}
	valid := []T{}
	var invalidErr error
	for _, item := range current {
		isValid := e.IsValid(item)
		if isValid {
			valid = append(valid, item)
			continue
		}
		invalidErr = errors.Join(invalidErr, e.InvalidStrategy.ProcessInvalid(item))
	}
	if invalidErr != nil {
		// we don't immediately return this error - even if we can't determine what objects are invalid
		// we still want to process as many valid objects as possible
		invalidErr = fmt.Errorf("unable to remove all invalid objects: %w", invalidErr)
	}
	needs := e.match(objs, valid)

	var validErr error
	for _, needObj := range needs {
		validErr = errors.Join(validErr, e.ValidStrategy.ProcessValid(needObj))
	}
	if validErr != nil {
		validErr = fmt.Errorf("unable to process all valid objects: %w", validErr)
	}

	return errors.Join(invalidErr, validErr)
}

// match returns a list of items from want which aren't in exists, using ObjectIdentifier to match between the two slices
func (e *Ensurer[T]) match(want []T, exists []T) []T {
	existsMap := map[string]struct{}{}
	for _, existsItem := range exists {
		existsMap[e.ObjectIdentifier(existsItem)] = struct{}{}
	}
	needs := []T{}
	for _, wantItem := range want {
		key := e.ObjectIdentifier(wantItem)
		if _, ok := existsMap[key]; !ok {
			needs = append(needs, wantItem)
		}
	}
	return needs
}
