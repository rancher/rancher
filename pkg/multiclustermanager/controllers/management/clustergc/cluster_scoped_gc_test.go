package clustergc

import (
	"context"
	"testing"

	"github.com/rancher/norman/lifecycle"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func TestCleanFinalizersGeneric(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		object      *unstructured.Unstructured
		wantFinal   []string
	}{
		{
			name:        "basic case",
			clusterName: "test",
			object: finalizerFactory(
				lifecycle.ScopedFinalizerKey + "blah" + "_" + "test",
			),
			wantFinal: []string{},
		},
		{
			"DontRemoveUnrelated",
			"a",
			finalizerFactory(
				lifecycle.ScopedFinalizerKey+"App"+"_"+"b",
				lifecycle.ScopedFinalizerKey+"App"+"_"+"a",
			),
			[]string{lifecycle.ScopedFinalizerKey + "App" + "_" + "b"},
		},
		{
			"NoFinalizers",
			"a",
			&unstructured.Unstructured{},
			nil,
		},
		{
			"DontAffectNonScoped",
			"a",
			finalizerFactory("controller.cattle.io/" + "App" + "_" + "a"),
			[]string{"controller.cattle.io/" + "App" + "_" + "a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			object, err := cleanFinalizers(tt.clusterName, tt.object, mockDynamicResourceInterface{})
			if err != nil {
				t.Errorf("cleanFinalizersGeneric() error = %v", err)
			}
			md, err := meta.Accessor(object)
			if err != nil {
				t.Errorf("cleanFinalizersGeneric() error = %v", err)
			}
			finalizers := md.GetFinalizers()
			assert.Equal(t, tt.wantFinal, finalizers)
		})
	}
}

// use meta accessors to set finalizer values
func finalizerFactory(finals ...string) *unstructured.Unstructured {

	randomStr := "nameofType"
	metadata := &metav1.ObjectMeta{
		Name:       randomStr,
		Finalizers: finals,
	}
	unstruct := &unstructured.Unstructured{}
	err := setObjectMeta(unstruct, metadata)
	if err != nil {
		panic(err)
	}
	return unstruct

}

func setObjectMeta(u *unstructured.Unstructured, objectMeta *metav1.ObjectMeta) error {
	if objectMeta == nil {
		unstructured.RemoveNestedField(u.UnstructuredContent(), "metadata")
		return nil
	}
	metadata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectMeta)
	if err != nil {
		return err
	}
	if u.Object == nil {
		u.Object = make(map[string]interface{})
	}
	u.Object["metadata"] = metadata
	return nil
}

type mockDynamicResourceInterface struct{}

func (mockDynamicResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return obj, nil
}

func (mockDynamicResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (mockDynamicResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (mockDynamicResourceInterface) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (mockDynamicResourceInterface) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("implement me")
}

func (mockDynamicResourceInterface) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (mockDynamicResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (mockDynamicResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (mockDynamicResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}
