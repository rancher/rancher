package machineprovisioning

import (
	"fmt"
	"strings"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// Test_Operation_SetB_MP_EtcdSnapshotOperationsOnNewNode uses Minio as an object store to store S3 snapshots. It creates a 2 node machine provisioned cluster with a controlplane+worker and
// etcd node, creates a configmap, takes a snapshot of the cluster, deletes the configmap, then scales the etcd node away.
// The old init etcd Machine stays protected, so scaling the etcd pool back up lets the new node join through it and take over.
// The snapshot is then restored onto the new node.
func Test_Operation_SetB_MP_EtcdSnapshotOperationsOnNewNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	newNs, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	osInfo, err := objectstore.GetObjectStore(clients, newNs.Name, "store0", "s3snapshots")
	if err != nil {
		t.Fatal(err)
	}

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp-etcd-snapshot-operations-on-new-node",
			Namespace: newNs.Name,
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
						S3: &rkev1.ETCDSnapshotS3{
							Endpoint:            osInfo.Endpoint,
							EndpointCA:          osInfo.Cert,
							Bucket:              osInfo.Bucket,
							CloudCredentialName: osInfo.CloudCredentialName,
							Folder:              "testfolder",
						},
					},
				},
				MachinePools: []provisioningv1.RKEMachinePool{
					{
						EtcdRole: true,
						Quantity: &defaults.One,
					},
					{
						ControlPlaneRole: true,
						WorkerRole:       true,
						Quantity:         &defaults.One,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: map[string]string{
			"test": "wow",
		},
	}

	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, "s3")
	assert.NotNil(t, snapshot)

	oldInit := findInitEtcdMachine(t, clients, c)

	// Scale etcd nodes to 0
	c, err = operations.Scale(clients, c, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating the last etcd member is protected", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "last etcd member is protected"), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// The old init Machine must stay held (deleting, not removed) until the replacement takes over.
	held, err := clients.CAPI.Machine().Get(oldInit.Namespace, oldInit.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if held.DeletionTimestamp.IsZero() {
		t.Fatalf("old init etcd machine %s is not deleting", held.Name)
	}

	// Deleting Machines still count toward the MachineSet size, so scaling straight to 1 creates nothing.
	// Scale above the held etcd Machines to create one replacement, then scale back to 1 so the deleting Machines are removed first.
	ml, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}
	heldUIDs := map[types.UID]bool{}
	for i := range ml.Items {
		if ml.Items[i].Labels[capr.EtcdRoleLabel] == "true" {
			heldUIDs[ml.Items[i].UID] = true
		}
	}
	heldCount := len(heldUIDs)

	c, err = operations.Scale(clients, c, 0, int32(heldCount+1), false)
	if err != nil {
		t.Fatal(err)
	}

	var replacement *capi.Machine
	for deadline := time.Now().Add(10 * time.Minute); replacement == nil; time.Sleep(2 * time.Second) {
		if time.Now().After(deadline) {
			t.Fatal("no replacement etcd machine was created")
		}
		ml, err := cluster.Machines(clients, c)
		if err != nil {
			t.Fatal(err)
		}
		for i := range ml.Items {
			m := &ml.Items[i]
			if m.Labels[capr.EtcdRoleLabel] == "true" && m.DeletionTimestamp.IsZero() && !heldUIDs[m.UID] {
				replacement = m
				break
			}
		}
	}

	// Scale etcd nodes to 1
	c, err = operations.Scale(clients, c, 0, 1, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := waitForMachineGone(clients, oldInit, 15*time.Minute); err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	newInit := findInitEtcdMachine(t, clients, c)
	if newInit.UID == oldInit.UID {
		t.Fatal("the old init etcd machine is still the init node")
	}
	if newInit.UID != replacement.UID {
		t.Fatalf("the new init etcd machine %s is not the replacement %s", newInit.Name, replacement.Name)
	}

	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 2)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

// findInitEtcdMachine returns the etcd Machine whose plan Secret carries the init-node label.
func findInitEtcdMachine(t *testing.T, clients *clients.Clients, c *provisioningv1.Cluster) *capi.Machine {
	t.Helper()
	ml, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}
	for i := range ml.Items {
		m := &ml.Items[i]
		if m.Labels[capr.EtcdRoleLabel] != "true" {
			continue
		}
		s, err := initProtectedPlanSecret(clients, m)
		if err != nil {
			t.Fatal(err)
		}
		if s.Labels[capr.InitNodeLabel] == "true" {
			return m
		}
	}
	t.Fatal("no etcd machine carries the init-node label")
	return nil
}

// waitForMachineGone polls until the Machine with the same UID is gone, or fails after timeout.
func waitForMachineGone(clients *clients.Clients, m *capi.Machine, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		current, err := clients.CAPI.Machine().Get(m.Namespace, m.Name, metav1.GetOptions{})
		if apierror.IsNotFound(err) || (err == nil && current.UID != m.UID) {
			return nil
		}
		if err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("machine %s still exists after %s", m.Name, timeout)
		}
		time.Sleep(5 * time.Second)
	}
}
