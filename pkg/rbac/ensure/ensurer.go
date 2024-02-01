package ensure

import (
	"errors"
	"fmt"

	"github.com/rancher/wrangler/pkg/generic"
)

// Ensurer is used to ensure that a group of objects has been created in k8s, and that all related corrupted versions
// of the object are properly processed.
type Ensurer[T generic.RuntimeMetaObject] struct {
	// CurrentStrategy is the strategy for retrieving existing objects
	CurrentStrategy ExistingStrategy[T]
	// MatchStrategy compares the requested set of objects with the currently existing, valid
	// set of objects
	MatchStrategy func(want []T, current []T) ([]T, error)
	// IsValid is used to determine if a current object should be processed by ValidStrategy
	// or by InvalidStrategy
	IsValid func(obj T) bool
	// InvalidStrategy describes how to process invalid objects
	InvalidStrategy InvalidObjStrategy[T]
	// ValidStrategy describes how to process valid objects
	ValidStrategy ValidObjStrategy[T]
}

// Ensure uses CurrentStrategy to determine what objects exist, IsValid to determine which objects are valid,
// InvalidStrategy to process the invalid objects, MatchStrategy to find out what objs are currently covered by valid
// objects, and ValidStrategy to process the the uncovered valid objects. Will return an error if it is not able to
// process any valid/invalid objects, but only after trying to process everything that can be confirmed.
func (e *Ensurer[T]) Ensure(objs []T) error {
	current, err := e.CurrentStrategy.GetCurrent()
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
	needs, err := e.MatchStrategy(objs, valid)
	if err != nil {
		return errors.Join(invalidErr, fmt.Errorf("unable to determine which objects are needed: %w", err))
	}

	var validErr error
	for _, needObj := range needs {
		validErr = errors.Join(validErr, e.ValidStrategy.ProcessValid(needObj))
	}
	if validErr != nil {
		validErr = fmt.Errorf("unable to process all valid objects: %w", validErr)
	}

	return errors.Join(invalidErr, validErr)
}
