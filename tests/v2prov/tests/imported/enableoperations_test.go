package imported

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Operation_SetE_ImportedEnableOperations(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	ns, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	registryCACert, err := registry.EnsureRegistryCache(clients)
	if err != nil {
		t.Fatal(err)
	}

	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, defaults.SomeK8sVersion, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	}, nil, registryCACert)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, pods, 1)

	mgmtCluster, err := cluster.NewImported(clients, &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"operations.cattle.io/ops-enabled": "false",
			},
			GenerateName: "c-",
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    "test-imported-snapshot",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ensure the operations are not enabled
	assert.Equal(t, "false", mgmtCluster.Annotations["operations.cattle.io/ops-enabled"], "expected operation to be disabled")

	// get the cluster and ensure the operations are not enabled
	mgmtCluster, err = clients.Mgmt.Cluster().Get(mgmtCluster.Name, metav1.GetOptions{})
	require.NoError(t, err, "expected no error getting mgmt cluster")

	// ensure no system-agents are running in the downstream cluster

	// validate no SUC plan

	// todo(jhyde): validate no rancher-system-agent

	// attempt to create an operation, should be aborted/rejected

	// enable operations

	// ensure everything correctly installed

	//
}
