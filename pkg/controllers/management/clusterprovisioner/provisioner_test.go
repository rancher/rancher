package clusterprovisioner

import (
	"testing"

	"github.com/rancher/norman/controller"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	appsfakes "github.com/rancher/rancher/pkg/generated/norman/apps/v1/fakes"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
)

func TestCreateWaitingForVersionDoesNotRewriteStatus(t *testing.T) {
	for _, name := range []string{"local", "c-imported"} {
		t.Run(name, func(t *testing.T) {
			stored := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
			stored.Status.Driver = apimgmtv3.ClusterDriverImported
			apimgmtv3.ClusterConditionPending.True(stored)
			apimgmtv3.ClusterConditionProvisioned.True(stored)
			apimgmtv3.ClusterConditionProvisioned.LastUpdated(stored, "2026-01-01T00:00:00Z")
			apimgmtv3.ClusterConditionWaiting.Unknown(stored)
			writes := 0
			p := &Provisioner{
				Clusters: &v3fakes.ClusterInterfaceMock{
					UpdateStatusFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
						writes++
						stored.Status = *c.Status.DeepCopy()
						return stored.DeepCopy(), nil
					},
					UpdateFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
						stored.Spec = *c.Spec.DeepCopy()
						return stored.DeepCopy(), nil
					},
				},
				DaemonsetLister: &appsfakes.DaemonSetListerMock{GetFunc: func(namespace, name string) (*appsv1.DaemonSet, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "daemonsets"}, name)
				}},
				NodeLister: &v3fakes.NodeListerMock{ListFunc: func(string, labels.Selector) ([]*apimgmtv3.Node, error) {
					return []*apimgmtv3.Node{{}}, nil
				}},
			}
			writesAfterFirstAttempt := 0
			for attempt := range 2 {
				_, err := p.Create(stored.DeepCopy())
				var waiting *controller.ForgetError
				require.ErrorAs(t, err, &waiting)
				require.Equal(t, "waiting for full cluster configuration", waiting.Error())
				if attempt == 0 {
					writesAfterFirstAttempt = writes
				}
			}
			require.Equal(t, writesAfterFirstAttempt, writes, "waiting for version must not rewrite unchanged Provisioned status on every retry")

			// Once cluster-stats discovers the version, the create step can finish.
			stored.Status.Version = &version.Info{GitVersion: "v1.36.6+rke2r1"}
			obj, err := p.Create(stored.DeepCopy())
			require.NoError(t, err)
			require.Equal(t, apimgmtv3.ClusterDriverRke2, obj.(*apimgmtv3.Cluster).Status.Driver)
			require.Equal(t, apimgmtv3.ClusterDriverRke2, stored.Status.Driver)
			require.True(t, apimgmtv3.ClusterConditionProvisioned.IsTrue(stored))
			require.Equal(t, "v1.36.6+rke2r1", stored.Spec.Rke2Config.Version)
		})
	}
}

func TestUpdatedPreservesStatusAcrossSpecUpdate(t *testing.T) {
	stored := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{
		Name:        "c-imported",
		Annotations: map[string]string{KontainerEngineUpdate: "updated"},
	}}
	stored.Status.Driver = apimgmtv3.ClusterDriverImported
	stored.Status.Version = &version.Info{GitVersion: "v1.36.6+k3s1"}
	p := &Provisioner{
		Clusters: &v3fakes.ClusterInterfaceMock{
			UpdateFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
				stored.Spec = *c.Spec.DeepCopy()
				return stored.DeepCopy(), nil
			},
			UpdateStatusFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
				stored.Status = *c.Status.DeepCopy()
				return stored.DeepCopy(), nil
			},
		},
		NodeLister: &v3fakes.NodeListerMock{ListFunc: func(string, labels.Selector) ([]*apimgmtv3.Node, error) {
			return []*apimgmtv3.Node{{}}, nil
		}},
	}
	_, err := p.Updated(stored.DeepCopy())
	require.NoError(t, err)
	require.Equal(t, apimgmtv3.ClusterDriverK3s, stored.Status.Driver)
	require.Equal(t, "v1.36.6+k3s1", stored.Spec.K3sConfig.Version)
	require.True(t, apimgmtv3.ClusterConditionProvisioned.IsTrue(stored))
	require.True(t, apimgmtv3.ClusterConditionPending.IsTrue(stored))
	require.True(t, apimgmtv3.ClusterConditionUpdated.IsTrue(stored))
}
