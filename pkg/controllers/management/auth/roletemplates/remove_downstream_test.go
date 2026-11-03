package roletemplates

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// removingClusters are clusters being removed whose downstream resources are left in place: the cluster
// manager isn't set up in these tests, so asking it for a user context would fail the test.
func removingClusters() map[string]*v3.Cluster {
	now := metav1.Now()
	removing := func(mutate func(*v3.Cluster)) *v3.Cluster {
		c := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", DeletionTimestamp: &now},
			Status:     v3.ClusterStatus{Driver: v3.ClusterDriverImported, APIEndpoint: "https://10.0.0.1", CACert: "ca"},
		}
		mutate(c)
		return c
	}
	return map[string]*v3.Cluster{
		"created by a provisioning cluster": removing(func(c *v3.Cluster) {
			c.Annotations = map[string]string{"provisioning.cattle.io/administrated": "true"}
		}),
		"never connected": removing(func(c *v3.Cluster) { c.Status.CACert = "" }),
		"past removing its role template bindings": removing(func(c *v3.Cluster) {
			v3.ClusterConditionRoleTemplateBindingsRemoved.False(c)
		}),
	}
}

func TestCRTBDeleteDownstreamResourcesLeavesThemWhenTheClusterIsRemoved(t *testing.T) {
	for name, cluster := range removingClusters() {
		t.Run(name, func(t *testing.T) {
			clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](gomock.NewController(t))
			clusters.EXPECT().Get("c-m-test", gomock.Any()).Return(cluster, nil)
			c := &crtbHandler{clusterController: clusters}

			err := c.deleteDownstreamResources(&v3.ClusterRoleTemplateBinding{
				ObjectMeta:  metav1.ObjectMeta{Namespace: "c-m-test", Name: "crtb-a"},
				ClusterName: "c-m-test",
				UserName:    "u-1",
			}, true)

			assert.NoError(t, err)
		})
	}
}

func TestPRTBDeleteDownstreamResourcesLeavesThemWhenTheClusterIsRemoved(t *testing.T) {
	for name, cluster := range removingClusters() {
		t.Run(name, func(t *testing.T) {
			clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](gomock.NewController(t))
			clusters.EXPECT().Get("c-m-test", gomock.Any()).Return(cluster, nil)
			p := &prtbHandler{clusterController: clusters}

			err := p.deleteDownstreamResources(&v3.ProjectRoleTemplateBinding{
				ObjectMeta:  metav1.ObjectMeta{Namespace: "c-m-test-p-1", Name: "prtb-x"},
				ProjectName: "c-m-test:p-1",
				UserName:    "u-1",
			}, true)

			assert.NoError(t, err)
		})
	}
}
