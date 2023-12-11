package machineprovisioning

import (
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/operations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Operation_SetA_MP_EncryptionKeyRotation(t *testing.T) {
	// Encryption Key rotation is only possible with "stock configuration" on RKE2.
	if strings.ToLower(os.Getenv("DIST")) != "rke2" {
		t.Skip("encryption key rotation")
	}

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mp-encryption-key-rotation-operations",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				MachinePools: []provisioningv1.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	operations.RunRotateEncryptionKeysTest(t, clients, c)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
