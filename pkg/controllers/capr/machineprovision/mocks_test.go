package machineprovision

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type dynamicControllerFake struct {
	EnqueueAfterCalled int
}

func (d dynamicControllerFake) Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error) {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"dataSecretName": "my-secret",
			},
		},
	}, nil
}

func (d dynamicControllerFake) Update(obj runtime.Object) (runtime.Object, error) {
	return nil, nil
}

func (d dynamicControllerFake) UpdateStatus(obj runtime.Object) (runtime.Object, error) {
	return obj, nil
}

func (d dynamicControllerFake) Enqueue(gvk schema.GroupVersionKind, namespace, name string) error {
	return nil
}

func (d *dynamicControllerFake) EnqueueAfter(gvk schema.GroupVersionKind, namespace, name string, delay time.Duration) error {

	d.EnqueueAfterCalled += 1
	return nil
}
