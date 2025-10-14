package alibaba

import (
	"context"
	"embed"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/types/config/dialer"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"

	"github.com/ghodss/yaml"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	test "github.com/rancher/rancher/pkg/controllers/management/alibaba/mocks"
	"github.com/rancher/rancher/pkg/controllers/management/clusteroperator"
	"github.com/rancher/rancher/pkg/types/config/dialer/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

//go:embed test/*
var testFs embed.FS

const (
	MockDefaultClusterFilename          = "test/onclusterchange_default.yaml"
	MockCreateClusterFilename           = "test/onclusterchange_create.yaml"
	MockActiveClusterFilename           = "test/onclusterchange_active.yaml"
	MockUpdateClusterFilename           = "test/onclusterchange_update.yaml"
	MockAliClusterConfigFilename        = "test/updatealicc.json"
	MockAliClusterConfigClusterFilename = "test/updatealicc.yaml"
	MockBuildAliCCCreateObjectFilename  = "test/buildalicccreateobject.json"
	MockDefaultAliClusterConfigFilename = "test/onclusterchange_alicc_default.json"
	MockCreateAliClusterConfigFilename  = "test/onclusterchange_alicc_create.json"
	MockActiveAliClusterConfigFilename  = "test/onclusterchange_alicc_active.json"
	MockUpdateAliClusterConfigFilename  = "test/onclusterchange_alicc_update.json"
	MockAliClusterConfigUpdatedFilename = "test/updatealicc_updated.json"
)

/*
* Test_onClusterChange
- cluster == nil. Return (nil nil)
- cluster.DeletionTimestamp or cluster.AliConfig == nil, return (nil nil)
- default phase
- create phase
- active phase
- update node pool phase
*/
func Test_onClusterChange_ClusterIsNil(t *testing.T) {
	var mockOperatorController aliOperatorController
	cluster, _ := mockOperatorController.onClusterChange("", nil)
	if cluster != nil {
		t.Errorf("cluster should have returned nil")
	}
}

func Test_onClusterChange_AliConfigIsNil(t *testing.T) {
	var mockOperatorController aliOperatorController
	mockCluster := &v3.Cluster{
		Spec: v3.ClusterSpec{
			AliConfig: nil,
		},
	}

	cluster, _ := mockOperatorController.onClusterChange("", mockCluster)
	if !reflect.DeepEqual(cluster, mockCluster) {
		t.Errorf("cluster should have returned with no update")
	}
}

func Test_onClusterChange_Default(t *testing.T) {

	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	resourceMock := test.NewMockResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)

	discoveryMock.EXPECT().ServerResourcesForGroupVersion("ali.cattle.io/v1").Return(&v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Name: "aliclusterconfigs"},
		},
	}, nil)
	dynamicMock.EXPECT().Namespace(gomock.Any()).Return(resourceMock)
	mockAliCC, err := getMockAliClusterConfig(MockDefaultAliClusterConfigFilename)
	assert.NoError(t, err)
	resourceMock.EXPECT().Get(gomock.Any(), "c-abcde", gomock.Any()).Return(mockAliCC, nil)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)

	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	// run test
	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// validate results
	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsUnknown(cluster) {
		t.Errorf("provisioned status should be Unknown and cluster returned successfully")
	}
}

func Test_onClusterChange_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	resourceMock := test.NewMockResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)

	discoveryMock.EXPECT().ServerResourcesForGroupVersion("ali.cattle.io/v1").Return(&v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Name: "aliclusterconfigs"},
		},
	}, nil)
	dynamicMock.EXPECT().Namespace(gomock.Any()).Return(resourceMock)
	mockAliCC, err := getMockAliClusterConfig(MockCreateAliClusterConfigFilename)
	assert.NoError(t, err)
	resourceMock.EXPECT().Get(gomock.Any(), "c-abcde", gomock.Any()).Return(mockAliCC, nil)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)
	mockCluster, err := getMockV3Cluster(MockCreateClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsUnknown(cluster) {
		t.Errorf("provisioned status should be Unknown and cluster returned successfully")
	}
}

func Test_onClusterChange_Active(t *testing.T) {
	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	resourceMock := test.NewMockResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)

	discoveryMock.EXPECT().ServerResourcesForGroupVersion("ali.cattle.io/v1").Return(&v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Name: "aliclusterconfigs"},
		},
	}, nil)
	dynamicMock.EXPECT().Namespace(gomock.Any()).Return(resourceMock)
	mockAliCC, err := getMockAliClusterConfig(MockActiveAliClusterConfigFilename)
	assert.NoError(t, err)
	resourceMock.EXPECT().Get(gomock.Any(), "c-abcde", gomock.Any()).Return(mockAliCC, nil)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsTrue(cluster) || !capr.Updated.IsTrue(cluster) {
		t.Errorf("provisioned and updated status should be True and cluster returned successfully")
	}
}

func Test_onClusterChange_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	resourceMock := test.NewMockResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)

	discoveryMock.EXPECT().ServerResourcesForGroupVersion("ali.cattle.io/v1").Return(&v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Name: "aliclusterconfigs"},
		},
	}, nil)
	dynamicMock.EXPECT().Namespace(gomock.Any()).Return(resourceMock)
	mockAliCC, err := getMockAliClusterConfig(MockUpdateAliClusterConfigFilename)
	assert.NoError(t, err)
	resourceMock.EXPECT().Get(gomock.Any(), "c-abcde", gomock.Any()).Return(mockAliCC, nil)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)
	mockCluster, err := getMockV3Cluster(MockUpdateClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// check that cluster is updating
	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsTrue(cluster) || !capr.Updated.IsUnknown(cluster) {
		t.Errorf("provisioned status should be True, updated status should be Unknown and cluster returned successfully")
	}
}

/*
* Test_updateAliClusterConfig
  - success: Ali cluster tags are removed. Ali cluster is not immediately updated. Cluster sits in active for a few
    seconds, return (cluster nil)
*/
func Test_updateAliClusterConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	resourceMock := test.NewMockResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)
	watchMock := watch.NewFake()

	discoveryMock.EXPECT().ServerResourcesForGroupVersion("ali.cattle.io/v1").Return(&v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Name: "aliclusterconfigs"},
		},
	}, nil).AnyTimes()
	dynamicMock.EXPECT().Namespace(gomock.Any()).Return(resourceMock).AnyTimes()
	mockAliCC, err := getMockAliClusterConfig(MockAliClusterConfigFilename)
	assert.NoError(t, err)
	resourceMock.EXPECT().Get(gomock.Any(), "c-abcde", gomock.Any()).Return(mockAliCC, nil).AnyTimes()
	resourceMock.EXPECT().List(context.TODO(), v1.ListOptions{}).Return(&unstructured.UnstructuredList{}, nil)
	resourceMock.EXPECT().Watch(context.TODO(), gomock.Any()).Return(watchMock, nil)
	resourceMock.EXPECT().Update(context.TODO(), gomock.Any(), gomock.Any()).Return(mockAliCC, nil)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)
	mockCluster, err := getMockV3Cluster(MockAliClusterConfigClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	mockAliClusterConfig, err := getMockAliClusterConfig(MockAliClusterConfigFilename)
	if err != nil {
		t.Errorf("error getting mock ali cluster config: %s", err)
	}

	go func() {
		watchMock.Action(watch.Modified, mockAliClusterConfig)
	}()

	// test remove tags from the cluster
	_, err = mockOperatorController.updateAliClusterConfig(&mockCluster, mockAliClusterConfig, nil)

	if err != nil {
		t.Errorf("error running updateAliClusterConfig: %s", err)
	}
}

/*
* Test_buildAliCCCreateObject
- success: AliClusterConfig object created, return (AliClusterConfig nil)
*/
func Test_buildAliCCCreateObject(t *testing.T) {
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	expected, err := getMockAliClusterConfig(MockBuildAliCCCreateObjectFilename)
	if err != nil {
		t.Errorf("error getting mock ali cluster config: %s", err)
	}

	alicc, err := buildAliCCCreateObject(&mockCluster)

	if err != nil {
		t.Errorf("error running buildAliCCCreateObject: %s", err)
	}
	if !reflect.DeepEqual(alicc, expected) {
		t.Errorf("AliClusterConfig object was not built as expected")
	}
}

/*
* Test_recordAppliedSpec
- success: set current spec as applied spec. Return (updated cluster err)
- success: AliConfig and Applied Spec AliConfig are equal. Return (cluster nil)
*/
func Test_recordAppliedSpec_Updated(t *testing.T) {
	// We use a mock cluster that is still provisioning and in an Unknown state, because that is when the applied spec
	// needs to be updated.
	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)

	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)

	if err != nil {
		t.Errorf("error running recordAppliedSpec: %s", err)
	}
	if cluster.Status.AppliedSpec.AliConfig == nil {
		t.Errorf("cluster Status.AppliedSpec should have been updated with AliConfig")
	}
}

func Test_recordAppliedSpec_NoUpdate(t *testing.T) {
	// A mock active cluster already has the AliConfig set on the applied spec, so no update is required.
	ctrl := gomock.NewController(t)
	dynamicMock := test.NewMockNamespaceableResourceInterface(ctrl)
	dialerMock := mocks.NewMockFactory(ctrl)
	discoveryMock := test.NewMockDiscoveryInterface(ctrl)
	mockOperatorController := getMockAliOperatorController(t, dynamicMock, dialerMock, discoveryMock)

	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)

	if err != nil {
		t.Errorf("error running recordAppliedSpec: %s", err)
	}
	if !reflect.DeepEqual(cluster.Status.AppliedSpec, mockCluster.Status.AppliedSpec) {
		t.Errorf("cluster Status.AppliedSpec should have no update and cluster returned successfully")
	}
}

func getMockAliOperatorController(t *testing.T, dynamicClient dynamic.NamespaceableResourceInterface, dialerFactory dialer.Factory, discovery discovery.DiscoveryInterface) aliOperatorController {
	t.Helper()
	ctrl := gomock.NewController(t)
	clusterMock := wranglerfake.NewMockNonNamespacedClientInterface[*apimgmtv3.Cluster, *apimgmtv3.ClusterList](ctrl)
	clusterMock.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			return c, nil
		},
	).AnyTimes()

	return aliOperatorController{
		OperatorController: clusteroperator.OperatorController{
			ClusterEnqueueAfter:  func(name string, duration time.Duration) {},
			SecretsCache:         nil,
			Secrets:              nil,
			ProjectCache:         nil,
			NsClient:             nil,
			ClusterClient:        clusterMock,
			SystemAccountManager: nil,
			DynamicClient:        dynamicClient,
			ClientDialer:         dialerFactory,
			Discovery:            discovery,
		},
		secretClient: nil,
	}
}

// utility

func getMockV3Cluster(filename string) (apimgmtv3.Cluster, error) {
	var mockCluster apimgmtv3.Cluster

	// Read the embedded file
	cluster, err := testFs.ReadFile(filename)
	if err != nil {
		return mockCluster, err
	}
	// Unmarshal cluster yaml into a management v3 cluster object
	err = yaml.Unmarshal(cluster, &mockCluster)
	if err != nil {
		return mockCluster, err
	}

	return mockCluster, nil
}

func getMockAliClusterConfig(filename string) (*unstructured.Unstructured, error) {
	var aliClusterConfig *unstructured.Unstructured

	// Read the embedded file
	bytes, err := testFs.ReadFile(filename)
	if err != nil {
		return aliClusterConfig, err
	}
	// Unmarshal json into an unstructured cluster config object
	err = json.Unmarshal(bytes, &aliClusterConfig)
	if err != nil {
		return aliClusterConfig, err
	}

	return aliClusterConfig, nil
}
