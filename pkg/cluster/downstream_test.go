package cluster

import (
	"testing"

	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDownstreamCleanupRequired(t *testing.T) {
	tests := []struct {
		name    string
		cluster *apimgmtv3.Cluster
		want    bool
	}{
		{name: "imported", cluster: &apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverImported}}, want: true},
		{name: "imported k3s", cluster: &apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverK3s}}, want: true},
		{name: "imported rke2", cluster: &apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverRke2}}, want: true},
		{
			name: "created by a provisioning cluster",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"provisioning.cattle.io/administrated": "true"}},
				Status:     apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverImported},
			},
			want: false,
		},
		{
			name:    "hosted, imported",
			cluster: &apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverAKS, AKSStatus: apimgmtv3.AKSStatus{UpstreamSpec: &aksv1.AKSClusterConfigSpec{Imported: true}}}},
			want:    true,
		},
		{
			name:    "hosted, provisioned by Rancher",
			cluster: &apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverAKS, AKSStatus: apimgmtv3.AKSStatus{UpstreamSpec: &aksv1.AKSClusterConfigSpec{Imported: false}}}},
			want:    false,
		},
		{name: "no driver yet", cluster: &apimgmtv3.Cluster{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DownstreamCleanupRequired(tt.cluster))
		})
	}
}

func TestNeverConnected(t *testing.T) {
	assert.True(t, NeverConnected(&apimgmtv3.Cluster{}))
	assert.True(t, NeverConnected(&apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{APIEndpoint: "https://10.0.0.1"}}))
	assert.False(t, NeverConnected(&apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{APIEndpoint: "https://10.0.0.1", CACert: "ca"}}))
}

func TestSkipDownstreamCleanupOnRemoval(t *testing.T) {
	connectedImported := func() *apimgmtv3.Cluster {
		return &apimgmtv3.Cluster{Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverImported, APIEndpoint: "https://10.0.0.1", CACert: "ca"}}
	}

	skip, _ := SkipDownstreamCleanupOnRemoval(connectedImported())
	assert.False(t, skip, "a connected imported cluster is cleaned up while its role template bindings are removed")

	inProgress := connectedImported()
	apimgmtv3.ClusterConditionRoleTemplateBindingsRemoved.Unknown(inProgress)
	skip, _ = SkipDownstreamCleanupOnRemoval(inProgress)
	assert.False(t, skip, "still removing role template bindings")

	timedOut := connectedImported()
	apimgmtv3.ClusterConditionRoleTemplateBindingsRemoved.False(timedOut)
	skip, why := SkipDownstreamCleanupOnRemoval(timedOut)
	assert.True(t, skip, "the removal moved on")
	assert.Contains(t, why, "moved past")

	neverConnected := connectedImported()
	neverConnected.Status.CACert = ""
	skip, why = SkipDownstreamCleanupOnRemoval(neverConnected)
	assert.True(t, skip)
	assert.Contains(t, why, "never connected")

	v2prov := connectedImported()
	v2prov.Annotations = map[string]string{"provisioning.cattle.io/administrated": "true"}
	skip, why = SkipDownstreamCleanupOnRemoval(v2prov)
	assert.True(t, skip)
	assert.Contains(t, why, "removed with it")
}
