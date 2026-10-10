package networkpolicy

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newRecreatedCluster returns a cluster created again under the name c-m-test that, but for its UID, the
// handlers would act on given its network policy annotation.
func newRecreatedCluster(annotation string) *v3.Cluster {
	enable := true
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-new", Annotations: map[string]string{netPolAnnotation: annotation}},
		Spec:       v32.ClusterSpec{ClusterSpecBase: v32.ClusterSpecBase{EnableNetworkPolicy: &enable}},
	}
	v32.ClusterConditionProvisioned.True(cluster)
	v32.ClusterConditionReady.True(cluster)
	return cluster
}

// The handlers have no clients: the cluster created again under the same name must not be touched.
func TestClusterHandlersSkipAClusterCreatedAgainUnderTheSameName(t *testing.T) {
	t.Run("network policies", func(t *testing.T) {
		ch := &clusterHandler{
			cluster:          &config.UserContext{ClusterName: "c-m-test", ClusterUID: "uid-old"},
			clusterNamespace: "c-m-test",
		}

		// The annotation differs from the applied setting.
		out, err := ch.Sync("c-m-test", newRecreatedCluster("true"))

		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("network policy annotation", func(t *testing.T) {
		cn := &clusterNetAnnHandler{clusterNamespace: "c-m-test", clusterUID: "uid-old"}

		// The annotation differs from the spec.
		out, err := cn.Sync("c-m-test", newRecreatedCluster("false"))

		require.NoError(t, err)
		assert.Nil(t, out)
	})
}
