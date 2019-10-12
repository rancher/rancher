package cluster

import (
	"testing"

	"github.com/rancher/rke/cloudprovider/aws"
	"github.com/rancher/rke/cloudprovider/azure"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const testServiceNodePortRange = "10000-32769"

func initializeController() *controller {
	c := controller{
		nodeLister: &fakes.NodeListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Node, error) {
				return []*v3.Node{}, nil
			},
		},
	}
	return &c
}

func TestSetNodePortRange(t *testing.T) {
	c := initializeController()
	testCluster := v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "testCluster",
		},
	}
	testCluster.Spec.RancherKubernetesEngineConfig = &v3.RancherKubernetesEngineConfig{
		Services: v3.RKEConfigServices{
			KubeAPI: v3.KubeAPIService{
				ServiceNodePortRange: testServiceNodePortRange,
			},
		},
	}
	caps := v3.Capabilities{}
	caps, err := c.RKECapabilities(caps, *testCluster.Spec.RancherKubernetesEngineConfig, testCluster.Name)
	assert.Nil(t, err)
	assert.Equal(t, testServiceNodePortRange, caps.NodePortRange)
}

func TestLoadBalancerCapability(t *testing.T) {
	c := initializeController()
	lbCap := true
	testCluster := v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "testCluster",
		},
	}
	testCluster.Spec.RancherKubernetesEngineConfig = &v3.RancherKubernetesEngineConfig{}

	// map of cloud provider name to expected lb capability
	cloudProviderLBCapabilityMap := map[v3.CloudProvider]*bool{
		{}:                                   nil,
		{Name: aws.AWSCloudProviderName}:     &lbCap,
		{Name: azure.AzureCloudProviderName}: &lbCap,
	}
	for cloudProvider, expectedLB := range cloudProviderLBCapabilityMap {
		testCluster.Spec.RancherKubernetesEngineConfig.CloudProvider = cloudProvider
		caps := v3.Capabilities{}
		caps, err := c.RKECapabilities(caps, *testCluster.Spec.RancherKubernetesEngineConfig, testCluster.Name)
		assert.Nil(t, err)
		assert.Equal(t, expectedLB, caps.LoadBalancerCapabilities.Enabled)
	}
}

func TestIngressCapability(t *testing.T) {
	c := initializeController()
	rkeSpec := v3.ClusterSpec{
		ClusterSpecBase: v3.ClusterSpecBase{
			RancherKubernetesEngineConfig: &v3.RancherKubernetesEngineConfig{
				Ingress: v3.IngressConfig{
					Provider: NginxIngressProvider,
				},
			},
		},
	}
	testClusters := []v3.Cluster{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "clusterWithNginx",
			},
			Spec: rkeSpec,
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "clusterWithoutNginx",
			},
			Spec: rkeSpec,
		},
	}
	// don't set nginx as the ingress provider for second cluster
	testClusters[1].Spec.RancherKubernetesEngineConfig.Ingress.Provider = ""

	for _, testCluster := range testClusters {
		caps := v3.Capabilities{}
		caps, err := c.RKECapabilities(caps, *testCluster.Spec.RancherKubernetesEngineConfig, testCluster.Name)
		assert.Nil(t, err)
		assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.Ingress.Provider, caps.IngressCapabilities[0].IngressProvider)
	}
}
