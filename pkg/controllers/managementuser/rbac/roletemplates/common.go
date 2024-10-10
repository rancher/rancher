package roletemplates

import (
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/name"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	aggregatorSuffix = "aggregator"
)

// createOrUpdateResource creates or updates the given resource
//   - getResource is a func that returns a single object and an error
//   - obj is the resource to create or update
//   - client is the Wrangler client to use to get/create/update resource
//   - areResourcesTheSame is a func that compares two resources and returns (true, nil) if they are equal, and (false, T) when not the same where T is an updated resource
func createOrUpdateResource[T generic.RuntimeMetaObject, TList runtime.Object](obj T, client generic.NonNamespacedClientInterface[T, TList], getResource func(T) (T, error), areResourcesTheSame func(T, T) (bool, T)) error {
	// attempt to get the resource
	resource, err := getResource(obj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil
		}
		// resource doesn't exist, create it
		_, err = client.Create(obj)
		return err
	}

	// check that the existing resource is the same as the one we want
	if same, updatedResource := areResourcesTheSame(resource, obj); !same {
		// if it has changed, update it to the correct version
		_, err := client.Update(updatedResource)
		if err != nil {
			return err
		}
	}
	return nil
}

// addAggregatorSuffix appends the aggregation suffix to a string safely (ie <= 63 characters)
func addAggregatorSuffix(s string) string {
	return name.SafeConcatName(s + aggregatorSuffix)
}
