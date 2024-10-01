package eks

import (
	"context"
	"net"

	openapi2 "github.com/google/gnostic-models/openapiv2"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	meta1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	MockDefaultEksClusterConfigFilename = "test/onclusterchange_ekscc_default.json"
	MockCreateEksClusterConfigFilename  = "test/onclusterchange_ekscc_create.json"
	MockActiveEksClusterConfigFilename  = "test/onclusterchange_ekscc_active.json"
	MockUpdateEksClusterConfigFilename  = "test/onclusterchange_ekscc_update.json"
	MockEksClusterConfigUpdatedFilename = "test/updateeksclusterconfig_updated.json"
)

// mock interfaces

// mock dynamic client (to return a mock EksClusterConfig)

// Test 1 - cluster in default/unknown state. Get will return an EksClusterConfig with an unknown provisioning phase.
// The rest of the method signatures have to be implemented to mock the interface. There will be one mock of this
// interface for each test.

type MockNamespaceableResourceInterfaceDefault struct{}

func (m MockNamespaceableResourceInterfaceDefault) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) Namespace(s string) dynamic.ResourceInterface {
	return MockResourceInterfaceDefault{}
}

func (m MockNamespaceableResourceInterfaceDefault) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceDefault) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

type MockResourceInterfaceDefault struct{}

func (m MockResourceInterfaceDefault) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return getMockEksClusterConfig(MockDefaultEksClusterConfigFilename)
}

func (m MockResourceInterfaceDefault) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceDefault) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}
func (m MockResourceInterfaceDefault) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

// Test 2 - cluster in creating state

type MockNamespaceableResourceInterfaceCreate struct{}

func (m MockNamespaceableResourceInterfaceCreate) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) Namespace(s string) dynamic.ResourceInterface {
	return MockResourceInterfaceCreate{}
}

func (m MockNamespaceableResourceInterfaceCreate) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceCreate) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

type MockResourceInterfaceCreate struct{}

func (m MockResourceInterfaceCreate) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return getMockEksClusterConfig(MockCreateEksClusterConfigFilename)
}

func (m MockResourceInterfaceCreate) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceCreate) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

// Test 3 - cluster in active state

type MockNamespaceableResourceInterfaceActive struct{}

func (m MockNamespaceableResourceInterfaceActive) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) Namespace(s string) dynamic.ResourceInterface {
	return MockResourceInterfaceActive{}
}

func (m MockNamespaceableResourceInterfaceActive) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceActive) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

type MockResourceInterfaceActive struct{}

func (m MockResourceInterfaceActive) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceActive) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceActive) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceActive) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockResourceInterfaceActive) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockResourceInterfaceActive) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return getMockEksClusterConfig(MockActiveEksClusterConfigFilename)
}

func (m MockResourceInterfaceActive) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockResourceInterfaceActive) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockResourceInterfaceActive) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceActive) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	// TODO implement me
	panic("implement me")
}

func (m MockResourceInterfaceActive) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	// TODO implement me
	panic("implement me")
}

// Test 4 - cluster in update node pool state

type MockNamespaceableResourceInterfaceUpdate struct{}

func (m MockNamespaceableResourceInterfaceUpdate) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) Namespace(s string) dynamic.ResourceInterface {
	return MockResourceInterfaceUpdate{}
}

func (m MockNamespaceableResourceInterfaceUpdate) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceUpdate) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

type MockResourceInterfaceUpdate struct{}

func (m MockResourceInterfaceUpdate) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return getMockEksClusterConfig(MockUpdateEksClusterConfigFilename)
}

func (m MockResourceInterfaceUpdate) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, nil
}

func (m MockResourceInterfaceUpdate) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (m MockResourceInterfaceUpdate) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceUpdate) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

// Test UpdateEksClusterConfig

type MockNamespaceableResourceInterfaceEksCC struct{}

func (m MockNamespaceableResourceInterfaceEksCC) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) Namespace(s string) dynamic.ResourceInterface {
	return MockResourceInterfaceEksCC{}
}

func (m MockNamespaceableResourceInterfaceEksCC) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m MockNamespaceableResourceInterfaceEksCC) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

type MockResourceInterfaceEksCC struct{}

func (m MockResourceInterfaceEksCC) Create(ctx context.Context, obj *unstructured.Unstructured, options meta1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceEksCC) Update(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return getMockEksClusterConfig(MockEksClusterConfigUpdatedFilename)
}

func (m MockResourceInterfaceEksCC) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options meta1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceEksCC) Delete(ctx context.Context, name string, options meta1.DeleteOptions, subresources ...string) error {
	panic("implement me")
}

func (m MockResourceInterfaceEksCC) DeleteCollection(ctx context.Context, options meta1.DeleteOptions, listOptions meta1.ListOptions) error {
	panic("implement me")
}

func (m MockResourceInterfaceEksCC) Get(ctx context.Context, name string, options meta1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return getMockEksClusterConfig(MockEksClusterConfigUpdatedFilename)
}

func (m MockResourceInterfaceEksCC) List(ctx context.Context, opts meta1.ListOptions) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": "Eks.cattle.io/v1",
			"kind":       "EksClusterConfigList",
			"metadata":   map[string]interface{}{"resourceVersion": "142650"},
		},
		Items: nil,
	}, nil
}

func (m MockResourceInterfaceEksCC) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m MockResourceInterfaceEksCC) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options meta1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

// mock interface that returns a watch event (for updateEksClusterConfig test)

type MockInterface struct{}

func (m MockInterface) Stop() {}

func (m MockInterface) ResultChan() <-chan watch.Event {
	return make(chan watch.Event)
}

func (m MockResourceInterfaceEksCC) Watch(ctx context.Context, opts meta1.ListOptions) (watch.Interface, error) {
	return MockInterface{}, nil
}

func (m MockResourceInterfaceEksCC) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options meta1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

// mock cluster dialer

type MockFactory struct{}

func (m MockFactory) ClusterDialer(clusterName string, retryOnError bool) (dialer.Dialer, error) {
	// pass a dialer func to the client
	dialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, nil
	}
	return dialer, nil
}

func (m MockFactory) ClusterDialHolder(clusterName string, retryOnError bool) (*transport.DialHolder, error) {
	clusterDialer, err := m.ClusterDialer(clusterName, retryOnError)
	return &transport.DialHolder{Dial: clusterDialer}, err
}

func (m MockFactory) DockerDialer(clusterName, machineName string) (dialer.Dialer, error) {
	panic("implement me")
}

func (m MockFactory) NodeDialer(clusterName, machineName string) (dialer.Dialer, error) {
	panic("implement me")
}

type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

// mock discovery

type MockDiscovery struct{}

func (m MockDiscovery) RESTClient() rest.Interface {
	panic("implement me")
}

func (m MockDiscovery) ServerGroups() (*meta1.APIGroupList, error) {
	panic("implement me")
}

func (m MockDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*meta1.APIResourceList, error) {

	return &meta1.APIResourceList{
		TypeMeta:     meta1.TypeMeta{},
		GroupVersion: "",
		APIResources: []meta1.APIResource{
			{Name: "EksClusterConfig"},
			{Name: "status"}},
	}, nil
}

func (m MockDiscovery) ServerGroupsAndResources() ([]*meta1.APIGroup, []*meta1.APIResourceList, error) {
	panic("implement me")
}

func (m MockDiscovery) ServerPreferredResources() ([]*meta1.APIResourceList, error) {
	panic("implement me")
}

func (m MockDiscovery) ServerPreferredNamespacedResources() ([]*meta1.APIResourceList, error) {
	panic("implement me")
}

func (m MockDiscovery) ServerVersion() (*version.Info, error) {
	panic("implement me")
}

func (m MockDiscovery) OpenAPISchema() (*openapi2.Document, error) {
	panic("implement me")
}

func (m MockDiscovery) OpenAPIV3() openapi.Client {
	panic("implement me")
}

func (m MockDiscovery) WithLegacy() discovery.DiscoveryInterface {
	panic("implement me")
}
