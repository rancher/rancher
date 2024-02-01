package ensure

import (
	"github.com/rancher/wrangler/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// InvalidObjStrategy describes the strategy for how to process objects which are invalid.
type InvalidObjStrategy[T generic.RuntimeMetaObject] interface {
	ProcessInvalid(invalid T) error
}

// DeleteInvalidObjStrategy implements InvalidObjStrategy and deletes all invalid objects
type DeleteInvalidObjStrategy[T generic.RuntimeMetaObject, TList runtime.Object] struct {
	Client generic.ClientInterface[T, TList]
}

func (d *DeleteInvalidObjStrategy[T, TList]) ProcessInvalid(invalid T) error {
	return d.Client.Delete(invalid.GetNamespace(), invalid.GetName(), &metav1.DeleteOptions{})
}
