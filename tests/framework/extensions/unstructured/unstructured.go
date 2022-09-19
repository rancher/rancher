package unstructured

import (
	"github.com/rancher/rancher/pkg/api/scheme"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// MustToUnstructured is a helper function that converts a runtime.Object to an unstructured.Unstructured
// to be used by the dynamic client. The dynamic client only uses *unstructured.Unstructured when accessing specific resources.
// e.g. for namespaces
//
//	 namespace := &coreV1.Namespace{
//		    ObjectMeta: metav1.ObjectMeta{
//				Name:        "namespaceName",
//				Annotations: annotations,
//				Labels:      labels,
//			},
//	 }
//	 namespaceUnstructured := unstructured.MustToUnstructured(namespace)
func MustToUnstructured(obj runtime.Object) *unstructured.Unstructured {
	var out unstructured.Unstructured
	err := scheme.Scheme.Convert(obj, &out, nil)
	if err != nil {
		panic(err)
	}

	return &out
}
