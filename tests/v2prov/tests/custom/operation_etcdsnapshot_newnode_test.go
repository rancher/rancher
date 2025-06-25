package custom

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode creates a custom 2 node cluster with a controlplane+worker and
// etcd node, creates a configmap, takes a snapshot of the cluster, deletes the configmap, then deletes the etcd machine/node
// It then creates a new etcd node and restores from local snapshot file. This validates that it is possible to restore
// a snapshot on a completely new etcd node from file (without a corresponding snapshot file)
func Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-etcd-snapshot-operations-on-new-node",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --controlplane --worker", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tmpDirSeed, err := randomtoken.Generate()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := os.TempDir() + "/snapshot-" + tmpDirSeed[:32]

	// store the snapshots in a universal directory
	etcdSnapshotDir := []string{
		fmt.Sprintf("%s:/var/lib/rancher/%s/server/db/snapshots", tmpDir, capr.GetRuntime(c.Spec.KubernetesVersion)),
	}

	var etcdNodePodName string
	if etcdNode, err := systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd --node-name etcd-test-node", map[string]string{"custom-cluster-name": c.Name}, etcdSnapshotDir); err != nil {
		t.Fatal(err)
	} else {
		etcdNodePodName = etcdNode.Name
	}

	_, err = cluster.WaitForCreate(clients, c)
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

	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, "etcd-test-node")
	assert.NotNil(t, snapshot)

	err = clients.Core.Pod().Delete(c.Namespace, etcdNodePodName, &metav1.DeleteOptions{PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0]})
	if err != nil {
		t.Fatal(err)
	}

	if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
		return clients.Core.Pod().Get(c.Namespace, etcdNodePodName, metav1.GetOptions{})
	}); err != nil {
		t.Fatal(err)
	}

	// Delete the machine from the cluster too...
	oldEtcdMachines, err := clients.CAPI.Machine().List(c.Namespace, metav1.ListOptions{LabelSelector: capr.EtcdRoleLabel + "=true"})
	if err != nil {
		t.Fatal(err)
	}

	for _, machine := range oldEtcdMachines.Items {
		err = clients.CAPI.Machine().Delete(machine.Namespace, machine.Name, &metav1.DeleteOptions{PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0]})
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating insane cluster", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "waiting for at least one control plane, etcd, and worker node to be registered"), nil
	})

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name}, etcdSnapshotDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating restoration required", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "rkecontrolplane was already initialized but no etcd machines exist that have plans, indicating the etcd plane has been entirely replaced. Restoration from etcd snapshot is required."), nil
	})

	etcdMachines, err := clients.CAPI.Machine().List(c.Namespace, metav1.ListOptions{LabelSelector: capr.EtcdRoleLabel + "=true"})
	if err != nil {
		t.Fatal(err)
	}

	var newEtcdMachine *capi.Machine
	for _, machine := range etcdMachines.Items {
		if machine.DeletionTimestamp == nil {
			newEtcdMachine = &machine
		}
	}
	if newEtcdMachine == nil {
		t.Fatal("no etcd machine found")
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		snapshot2, err := clients.RKE.ETCDSnapshot().Get(snapshot.Namespace, snapshot.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			t.Logf("snapshot %s/%s not found, creating one...", snapshot.Namespace, snapshot.Name)
			snapshot = snapshot.DeepCopy()
			snapshot.Labels[capr.MachineIDLabel] = newEtcdMachine.Labels[capr.MachineIDLabel]
			snapshot.OwnerReferences = []metav1.OwnerReference{capr.ToOwnerReference(newEtcdMachine.TypeMeta, newEtcdMachine.ObjectMeta)}
			snapshot.UID = ""
			snapshot.ResourceVersion = ""
			snapshot, err = clients.RKE.ETCDSnapshot().Create(snapshot)
			return err
		}
		t.Logf("updating snapshot %s/%s", snapshot.Namespace, snapshot.Name)
		snapshot = snapshot2.DeepCopy()
		snapshot.Labels[capr.MachineIDLabel] = newEtcdMachine.Labels[capr.MachineIDLabel]
		snapshot.OwnerReferences = []metav1.OwnerReference{capr.ToOwnerReference(newEtcdMachine.TypeMeta, newEtcdMachine.ObjectMeta)}
		snapshot, err = clients.RKE.ETCDSnapshot().Update(snapshot)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.SnapshotFile.Name, cm, 2)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
