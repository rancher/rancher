package alibaba

import (
	"embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/clusteroperator"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

//go:embed test/*
var testFs embed.FS

func getMockAliOperatorController(t *testing.T, clusterState string) aliOperatorController {
	t.Helper()
	ctrl := gomock.NewController(t)
	clusterMock := fake.NewMockNonNamespacedClientInterface[*apimgmtv3.Cluster, *apimgmtv3.ClusterList](ctrl)
	clusterMock.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			return c, nil
		},
	).AnyTimes()
	var dynamicClient dynamic.NamespaceableResourceInterface

	switch clusterState {
	case "default":
		dynamicClient = MockNamespaceableResourceInterfaceDefault{}
	case "create":
		dynamicClient = MockNamespaceableResourceInterfaceCreate{}
	case "active":
		dynamicClient = MockNamespaceableResourceInterfaceActive{}
	case "update":
		dynamicClient = MockNamespaceableResourceInterfaceUpdate{}
	case "alicc":
		dynamicClient = MockNamespaceableResourceInterfaceAliCC{}
	default:
		dynamicClient = nil
	}

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
			ClientDialer:         MockFactory{},
			Discovery:            MockDiscovery{},
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
	var aksClusterConfig *unstructured.Unstructured

	// Read the embedded file
	bytes, err := testFs.ReadFile(filename)
	if err != nil {
		return aksClusterConfig, err
	}
	// Unmarshal json into an unstructured cluster config object
	err = json.Unmarshal(bytes, &aksClusterConfig)
	if err != nil {
		return aksClusterConfig, err
	}

	return aksClusterConfig, nil
}
