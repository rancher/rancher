package clustergc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rancher/norman/lifecycle"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
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

func (i mockDynamicResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (i mockDynamicResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

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

func TestRemoveWaitsForTheUserControllersToStop(t *testing.T) {
	// The user controllers add the finalizers back for as long as they run, so cleaning up before they
	// stop would leave finalizers nothing removes.
	var requeued []string
	gc := &gcLifecycle{enqueueAfter: func(_, name string, _ time.Duration) { requeued = append(requeued, name) }}
	cluster := &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}

	obj, err := gc.Remove(cluster)

	assert.ErrorIs(t, err, generic.ErrSkip, "the finalizer should be kept")
	assert.Same(t, cluster, obj)
	assert.Equal(t, []string{"c-m-test"}, requeued)
}

func TestRemoveDoesNotWaitOnceTheRemovalMovedOnWithoutTheReport(t *testing.T) {
	gc := &gcLifecycle{enqueueAfter: func(string, string, time.Duration) { t.Fatal("unexpected requeue") }}
	cluster := &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}
	apisv3.ClusterConditionUserControllersStopped.False(cluster)

	// The API refuses every request, so the cleanup itself fails: what matters is that it was attempted.
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	gc.mgmt = &config.ManagementContext{RESTConfig: rest.Config{Host: server.URL, QPS: 10000}}

	_, err := gc.Remove(cluster)

	require.Error(t, err)
	assert.NotErrorIs(t, err, generic.ErrSkip, "the cleanup should no longer wait")
	assert.NotZero(t, requests.Load())
}
