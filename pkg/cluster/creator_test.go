package cluster

import (
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// createdByAnnotations are the owner annotations wrangler apply sets on what an owner creates.
func createdByAnnotations(gvk, namespace, name string) map[string]string {
	return map[string]string{apply.LabelGVK: gvk, apply.LabelNamespace: namespace, apply.LabelName: name}
}

func TestGVKsMatchWhatApplyRecords(t *testing.T) {
	assert.Equal(t, "provisioning.cattle.io/v1, Kind=Cluster", ProvisioningClusterGVK)
	assert.Equal(t, "management.cattle.io/v3, Kind=Cluster", ManagementClusterGVK)
}

func TestCreatedBy(t *testing.T) {
	obj := &metav1.ObjectMeta{Annotations: createdByAnnotations(ProvisioningClusterGVK, "fleet-default", "foo")}

	assert.True(t, CreatedBy(obj, ProvisioningClusterGVK, "fleet-default", "foo"))
	assert.False(t, CreatedBy(obj, ProvisioningClusterGVK, "fleet-default", "bar"), "another name")
	assert.False(t, CreatedBy(obj, ProvisioningClusterGVK, "other", "foo"), "another namespace")
	assert.False(t, CreatedBy(obj, ManagementClusterGVK, "fleet-default", "foo"), "another kind")
	assert.False(t, CreatedBy(&metav1.ObjectMeta{}, ProvisioningClusterGVK, "fleet-default", "foo"), "no owner")
}

func TestCreatedByProvisioningCluster(t *testing.T) {
	mgmtCluster := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Annotations: createdByAnnotations(ProvisioningClusterGVK, "fleet-default", "foo")}}
	namespace, name, ok := CreatedByProvisioningCluster(mgmtCluster)
	assert.True(t, ok)
	assert.Equal(t, "fleet-default", namespace)
	assert.Equal(t, "foo", name)

	_, _, ok = CreatedByProvisioningCluster(&apimgmtv3.Cluster{})
	assert.False(t, ok, "a management cluster nothing created")

	_, _, ok = CreatedByProvisioningCluster(&apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Annotations: createdByAnnotations(ManagementClusterGVK, "", "c-m-test")}})
	assert.False(t, ok, "created by something other than a provisioning cluster")
}

func TestProvisioningInfrastructurePending(t *testing.T) {
	mgmtCluster := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", Annotations: createdByAnnotations(ProvisioningClusterGVK, "fleet-default", "foo")}}
	provCluster := func(finalizers ...string) func(string, string) (*provv1.Cluster, error) {
		return func(namespace, name string) (*provv1.Cluster, error) {
			assert.Equal(t, "fleet-default", namespace)
			assert.Equal(t, "foo", name)
			return &provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Finalizers: finalizers}}, nil
		}
	}

	pending, err := ProvisioningInfrastructurePending(mgmtCluster, provCluster(ProvisioningClusterRemoveFinalizer))
	require.NoError(t, err)
	assert.True(t, pending, "the provisioning cluster still tears down its machines")

	pending, err = ProvisioningInfrastructurePending(mgmtCluster, provCluster())
	require.NoError(t, err)
	assert.False(t, pending, "its machines are gone")

	pending, err = ProvisioningInfrastructurePending(mgmtCluster, func(string, string) (*provv1.Cluster, error) {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, "foo")
	})
	require.NoError(t, err)
	assert.False(t, pending, "the provisioning cluster is gone")

	pending, err = ProvisioningInfrastructurePending(&apimgmtv3.Cluster{}, func(string, string) (*provv1.Cluster, error) {
		t.Fatal("a management cluster nothing created has no provisioning cluster to look up")
		return nil, nil
	})
	require.NoError(t, err)
	assert.False(t, pending)

	_, err = ProvisioningInfrastructurePending(mgmtCluster, func(string, string) (*provv1.Cluster, error) {
		return nil, errors.New("boom")
	})
	assert.Error(t, err)
}
