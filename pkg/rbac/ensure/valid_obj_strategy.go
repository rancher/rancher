package ensure

import (
	"github.com/rancher/wrangler/pkg/generic"
	"k8s.io/apimachinery/pkg/runtime"
)

// ValidObjStrategy defines a strategy to process a valid object that is not yet covred by the existing objects
type ValidObjStrategy[T generic.RuntimeMetaObject] interface {
	ProcessValid(valid T) error
}

// CreateValidObjStrategy creates any valid object. Does not check if the object exists before creating.
type CreateValidObjStrategy[T generic.RuntimeMetaObject, TList runtime.Object] struct {
	Client generic.ClientInterface[T, TList]
}

func (c *CreateValidObjStrategy[T, TList]) ProcessValid(valid T) error {
	_, err := c.Client.Create(valid)
	return err
}
