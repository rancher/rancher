package healthsyncer

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateClusterHealthSkipsAClusterCreatedAgainUnderTheSameName(t *testing.T) {
	cluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-new"}}
	v32.ClusterConditionProvisioned.True(cluster)
	h := &HealthSyncer{
		clusterName: "c-m-test",
		clusterUID:  "uid-old",
		clusterLister: &fakes.ClusterListerMock{
			GetFunc: func(_, _ string) (*v3.Cluster, error) {
				return cluster, nil
			},
		},
		// No cluster client or downstream clients: the new cluster's health must not be written, nor checked
		// with the old cluster's clients.
	}

	assert.NoError(t, h.updateClusterHealth())
}
