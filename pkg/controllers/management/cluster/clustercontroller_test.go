package cluster

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rke/cloudprovider/aws"
	"github.com/rancher/rke/cloudprovider/azure"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
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
		v3.CloudProvider{}: nil,
		v3.CloudProvider{Name: aws.AWSCloudProviderName}:     &lbCap,
		v3.CloudProvider{Name: azure.AzureCloudProviderName}: &lbCap,
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

type capabilitiesTestCase struct {
	annotations  map[string]string
	capabilities v3.Capabilities
	result       v3.Capabilities
	errMsg       string
}

func TestOverrideCapabilities(t *testing.T) {
	assert := assert.New(t)

	fakeCapabilitiesSchema := types.Schema{
		ResourceFields: map[string]types.Field{
			"pspEnabled": {
				Type: "boolean",
			},
			"nodePortRange": {
				Type: "string",
			},
			"ingressCapabilities": {
				Type: "something",
			},
		},
	}
	tests := []capabilitiesTestCase{
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "pspEnabled"): "true",
			},
			capabilities: v3.Capabilities{},
			result: v3.Capabilities{
				PspEnabled: true,
			},
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "nodePortRange"): "9999",
			},
			capabilities: v3.Capabilities{},
			result: v3.Capabilities{
				NodePortRange: "9999",
			},
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "ingressCapabilities"): "[{\"customDefaultBackend\":true,\"ingressProvider\":\"asdf\"}]",
			},
			capabilities: v3.Capabilities{},
			result: v3.Capabilities{
				IngressCapabilities: []v3.IngressCapabilities{
					{
						CustomDefaultBackend: pointer.BoolPtr(true),
						IngressProvider:      "asdf",
					},
				},
			},
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "notarealcapability"): "something",
			},
			capabilities: v3.Capabilities{},
			errMsg:       "resource field [notarealcapability] from capabillities annotation not found",
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "pspEnabled"): "5",
			},
			capabilities: v3.Capabilities{},
			errMsg:       "strconv.ParseBool: parsing \"5\": invalid syntax",
		},
	}

	c := controller{
		capabilitiesSchema: &fakeCapabilitiesSchema,
	}
	for _, test := range tests {
		result, err := c.overrideCapabilities(test.annotations, test.capabilities)
		if err != nil {
			assert.Equal(test.errMsg, err.Error())
		} else {
			assert.True(reflect.DeepEqual(test.result, result))
		}
	}

	assert.Nil(nil)
}
