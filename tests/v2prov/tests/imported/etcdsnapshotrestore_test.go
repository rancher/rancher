package imported

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Test_Operation_SetD_ImportedETCDSnapshotRestore brings up an imported single-node cluster, creates
// a ConfigMap inside the downstream cluster, takes a snapshot via ETCDSnapshotSave, deletes the
// ConfigMap, runs ETCDSnapshotRestore, then verifies the ConfigMap returns. This exercises the
// operations-controller-driven restore path end-to-end against an imported cluster (where there is
// no provisioning.cattle.io Cluster or RKEControlPlane to drive the restore via spec).
func Test_Operation_SetD_ImportedETCDSnapshotRestore(t *testing.T) {
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

	// Single all-roles node — keep the topology minimal so the restore exercise stays focused on
	// the operation flow rather than multi-node coordination.
	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, defaults.SomeK8sVersion, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	}, nil, registryCACert)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, pods, 1)

	mgmtCluster, err := cluster.NewImported(clients, &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "c-",
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    "test-imported-restore",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	execKubectl := func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
			fmt.Sprintf("export %s && %s", kubectlEnv, cmd))
	}

	// Wait for the inner API server to be responsive before invoking the import command, mirroring
	// the save test.
	for i := 0; i < 60; i++ {
		_, err := execKubectl(t, "kubectl get nodes")
		if err == nil {
			break
		}
		if i == 59 {
			t.Fatalf("timed out waiting for %s API server to be ready: %v", distro, err)
		}
		time.Sleep(5 * time.Second)
	}

	out, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
		fmt.Sprintf("export %s && %s", kubectlEnv, importCmd))
	if err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleError(t, clients, mgmtCluster.Name, err)

	clusterRef := corev1.ObjectReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       mgmtCluster.Name,
	}

	// 1. Create a ConfigMap inside the downstream cluster — this is our proof-of-restore payload.
	cmName := "test-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10))
	cmValue := "wow"
	createCmd := fmt.Sprintf("kubectl create configmap %s --from-literal=test=%s", cmName, cmValue)
	if out, err := execKubectl(t, createCmd); err != nil {
		t.Fatalf("create configmap failed: %v\noutput: %s", err, out)
	}

	// 2. Take a snapshot of the cluster. Mark the cutoff time so we can pick the snapshot we just
	// took rather than any pre-existing one.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, clients, ns.Name, clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	waitForSnapshots(t, clients, mgmtCluster.Name, snapshotsValidAfter, 1)

	// 3. Wait for the back-populated ETCDSnapshot CR so we can pull the snapshot file name to
	// restore from. The snapshotbackpopulate controller mirrors downstream ETCDSnapshotFile resources
	// into the management cluster as rkev1.ETCDSnapshot CRs, in the namespace named for the cluster.
	snapshot := waitForBackpopulatedSnapshot(t, clients, mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}
	t.Logf("using snapshot file %q from %s", snapshot.SnapshotFile.Name, snapshot.Name)

	// 4. Delete the ConfigMap so the post-restore check is meaningful.
	deleteCmd := fmt.Sprintf("kubectl delete configmap %s", cmName)
	if out, err := execKubectl(t, deleteCmd); err != nil {
		t.Fatalf("delete configmap failed: %v\noutput: %s", err, out)
	}

	// 5. Drive the restore through to Succeeded.
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, clients, ns.Name, snapshot.SnapshotFile.Name, clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	// 6. After the restore finishes the cluster is restarted; the apiserver can take a while to come
	// back. Poll the ConfigMap until it's readable and matches our expected value.
	getCmd := fmt.Sprintf("kubectl get configmap %s -o jsonpath='{.data.test}'", cmName)
	var (
		gotValue string
		getErr   error
	)
	for i := 0; i < 60; i++ {
		gotValue, getErr = execKubectl(t, getCmd)
		if getErr == nil && strings.TrimSpace(gotValue) == cmValue {
			break
		}
		time.Sleep(5 * time.Second)
	}
	if getErr != nil {
		t.Fatalf("get configmap %s failed after restore: %v\noutput: %s", cmName, getErr, gotValue)
	}
	if strings.TrimSpace(gotValue) != cmValue {
		t.Fatalf("expected configmap %s value to be restored to %q, got %q", cmName, cmValue, gotValue)
	}
}

// Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesAllRoles brings up an imported single-node cluster, creates
// a ConfigMap inside the downstream cluster, takes a snapshot via ETCDSnapshotSave, deletes the
// ConfigMap, runs ETCDSnapshotRestore, then verifies the ConfigMap returns. This exercises the
// operations-controller-driven restore path end-to-end against an imported cluster (where there is
// no provisioning.cattle.io Cluster or RKEControlPlane to drive the restore via spec).
func Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesAllRoles(t *testing.T) {
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

	// Single all-roles node — keep the topology minimal so the restore exercise stays focused on
	// the operation flow rather than multi-node coordination.
	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, defaults.SomeK8sVersion, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 3},
	}, nil, registryCACert)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, pods, 3)

	mgmtCluster, err := cluster.NewImported(clients, &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "c-",
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    "test-imported-restore-3-nodes-all-roles",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	execKubectl := func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
			fmt.Sprintf("export %s && %s", kubectlEnv, cmd))
	}

	// Wait for the inner API server to be responsive before invoking the import command, mirroring
	// the save test.
	for i := 0; i < 60; i++ {
		_, err := execKubectl(t, "kubectl get nodes")
		if err == nil {
			break
		}
		if i == 59 {
			t.Fatalf("timed out waiting for %s API server to be ready: %v", distro, err)
		}
		time.Sleep(5 * time.Second)
	}

	out, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
		fmt.Sprintf("export %s && %s", kubectlEnv, importCmd))
	if err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleError(t, clients, mgmtCluster.Name, err)

	clusterRef := corev1.ObjectReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       mgmtCluster.Name,
	}

	// 1. Create a ConfigMap inside the downstream cluster — this is our proof-of-restore payload.
	cmName := "test-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10))
	cmValue := "wow"
	createCmd := fmt.Sprintf("kubectl create configmap %s --from-literal=test=%s", cmName, cmValue)
	if out, err := execKubectl(t, createCmd); err != nil {
		t.Fatalf("create configmap failed: %v\noutput: %s", err, out)
	}

	// 2. Take a snapshot of the cluster. Mark the cutoff time so we can pick the snapshot we just
	// took rather than any pre-existing one.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, clients, ns.Name, clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	waitForSnapshots(t, clients, mgmtCluster.Name, snapshotsValidAfter, 3)

	// 3. Wait for the back-populated ETCDSnapshot CR so we can pull the snapshot file name to
	// restore from. The snapshotbackpopulate controller mirrors downstream ETCDSnapshotFile resources
	// into the management cluster as rkev1.ETCDSnapshot CRs, in the namespace named for the cluster.
	// Purposefully restore from last etcd node.
	snapshot := waitForBackpopulatedSnapshot(t, clients, mgmtCluster.Name, "imported-node-2", snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}
	t.Logf("using snapshot %s", snapshot.Name)

	// 4. Delete the ConfigMap so the post-restore check is meaningful.
	deleteCmd := fmt.Sprintf("kubectl delete configmap %s", cmName)
	if out, err := execKubectl(t, deleteCmd); err != nil {
		t.Fatalf("delete configmap failed: %v\noutput: %s", err, out)
	}

	// 5. Drive the restore through to Succeeded.
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, clients, ns.Name, snapshot.Name, clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	// 6. After the restore finishes the cluster is restarted; the apiserver can take a while to come
	// back. Poll the ConfigMap until it's readable and matches our expected value.
	getCmd := fmt.Sprintf("kubectl get configmap %s -o jsonpath='{.data.test}'", cmName)
	var (
		gotValue string
		getErr   error
	)
	for i := 0; i < 60; i++ {
		gotValue, getErr = execKubectl(t, getCmd)
		if getErr == nil && strings.TrimSpace(gotValue) == cmValue {
			break
		}
		time.Sleep(5 * time.Second)
	}
	if getErr != nil {
		t.Fatalf("get configmap %s failed after restore: %v\noutput: %s", cmName, getErr, gotValue)
	}
	if strings.TrimSpace(gotValue) != cmValue {
		t.Fatalf("expected configmap %s value to be restored to %q, got %q", cmName, cmValue, gotValue)
	}
}

// Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesOneEach brings up an imported single-node cluster, creates
// a ConfigMap inside the downstream cluster, takes a snapshot via ETCDSnapshotSave, deletes the
// ConfigMap, runs ETCDSnapshotRestore, then verifies the ConfigMap returns. This exercises the
// operations-controller-driven restore path end-to-end against an imported cluster (where there is
// no provisioning.cattle.io Cluster or RKEControlPlane to drive the restore via spec).
func Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesOneEach(t *testing.T) {
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

	// Single all-roles node — keep the topology minimal so the restore exercise stays focused on
	// the operation flow rather than multi-node coordination.
	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, defaults.SomeK8sVersion, []cluster.ImportedNodePool{
		{ETCD: true, Quantity: 1},
		{ControlPlane: true, Quantity: 1},
		{Worker: true, Quantity: 1},
	}, nil, registryCACert)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, pods, 3)

	mgmtCluster, err := cluster.NewImported(clients, &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "c-",
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    "test-imported-restore-3-nodes-1-each",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	execKubectl := func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
			fmt.Sprintf("export %s && %s", kubectlEnv, cmd))
	}

	// Wait for the inner API server to be responsive before invoking the import command, mirroring
	// the save test.
	for i := 0; i < 60; i++ {
		_, err := execKubectl(t, "kubectl get nodes")
		if err == nil {
			break
		}
		if i == 59 {
			t.Fatalf("timed out waiting for %s API server to be ready: %v", distro, err)
		}
		time.Sleep(5 * time.Second)
	}

	out, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
		fmt.Sprintf("export %s && %s", kubectlEnv, importCmd))
	if err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleError(t, clients, mgmtCluster.Name, err)

	clusterRef := corev1.ObjectReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       mgmtCluster.Name,
	}

	// 1. Create a ConfigMap inside the downstream cluster — this is our proof-of-restore payload.
	cmName := "test-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10))
	cmValue := "wow"
	createCmd := fmt.Sprintf("kubectl create configmap %s --from-literal=test=%s", cmName, cmValue)
	if out, err := execKubectl(t, createCmd); err != nil {
		t.Fatalf("create configmap failed: %v\noutput: %s", err, out)
	}

	// 2. Take a snapshot of the cluster. Mark the cutoff time so we can pick the snapshot we just
	// took rather than any pre-existing one.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, clients, ns.Name, clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	waitForSnapshots(t, clients, mgmtCluster.Name, snapshotsValidAfter, 1)

	// 3. Wait for the back-populated ETCDSnapshot CR so we can pull the snapshot file name to
	// restore from. The snapshotbackpopulate controller mirrors downstream ETCDSnapshotFile resources
	// into the management cluster as rkev1.ETCDSnapshot CRs, in the namespace named for the cluster.
	// Purposefully restore from last etcd node.
	snapshot := waitForBackpopulatedSnapshot(t, clients, mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}
	t.Logf("using snapshot %s", snapshot.Name)

	// 4. Delete the ConfigMap so the post-restore check is meaningful.
	deleteCmd := fmt.Sprintf("kubectl delete configmap %s", cmName)
	if out, err := execKubectl(t, deleteCmd); err != nil {
		t.Fatalf("delete configmap failed: %v\noutput: %s", err, out)
	}

	// 5. Drive the restore through to Succeeded.
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, clients, ns.Name, snapshot.Name, clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	// todo(jhyde): ensure cluster is ready, and nodes are all present and happy

	// 6. After the restore finishes the cluster is restarted; the apiserver can take a while to come
	// back. Poll the ConfigMap until it's readable and matches our expected value.
	getCmd := fmt.Sprintf("kubectl get configmap %s -o jsonpath='{.data.test}'", cmName)
	var (
		gotValue string
		getErr   error
	)
	for i := 0; i < 60; i++ {
		gotValue, getErr = execKubectl(t, getCmd)
		if getErr == nil && strings.TrimSpace(gotValue) == cmValue {
			break
		}
		time.Sleep(5 * time.Second)
	}
	if getErr != nil {
		t.Fatalf("get configmap %s failed after restore: %v\noutput: %s", cmName, getErr, gotValue)
	}
	if strings.TrimSpace(gotValue) != cmValue {
		t.Fatalf("expected configmap %s value to be restored to %q, got %q", cmName, cmValue, gotValue)
	}
}
