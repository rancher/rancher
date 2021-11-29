package unstructured

import (
	"github.com/rancher/rancher/pkg/api/scheme"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func MustToUnstructured(obj runtime.Object) *unstructured.Unstructured {
	var out unstructured.Unstructured
	err := scheme.Scheme.Convert(obj, &out, nil)
	if err != nil {
		panic(err)
	}

	return &out
}
