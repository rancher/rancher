package cluster

import (
	"testing"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/fakes"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const testServiceNodePortRange = "10000-32769"

func TestSetNodePortRange(t *testing.T) {
	c := controller{
		nodeLister: &fakes.NodeListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Node, error) {
				return []*v3.Node{}, nil
			},
		},
	}

	testCluster := v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "testCluster",
		},
		Spec: v3.ClusterSpec{
			RancherKubernetesEngineConfig: &v3.RancherKubernetesEngineConfig{
				Services: v3.RKEConfigServices{
					KubeAPI: v3.KubeAPIService{
						ServiceNodePortRange: testServiceNodePortRange,
					},
				},
			},
		},
	}
	caps := v3.Capabilities{}
	caps, err := c.RKECapabilities(caps, *testCluster.Spec.RancherKubernetesEngineConfig, testCluster.Name)
	assert.Nil(t, err)
	assert.Equal(t, testServiceNodePortRange, caps.NodePortRange)
}
