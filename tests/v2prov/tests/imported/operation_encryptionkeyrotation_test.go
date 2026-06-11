package imported

import (
	"fmt"
	"os"
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
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_Operation_SetD_ImportedEncryptionKeyRotation(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) != "rke2" {
		t.Skip("encryption key rotation")
	}

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
			GenerateName: "c-",
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    "test-imported-encryption-key-rotation",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleEKRError(t, clients, ns.Name, mgmtCluster.Name, err)

	assert.NotEmpty(t, importCmd)

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	// Wait for the inner API server to be responsive, not just the kubeconfig file.
	for i := 0; i < 60; i++ {
		_, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name,
			"sh", "-c", fmt.Sprintf("export %s && kubectl get nodes", kubectlEnv))
		if err == nil {
			break
		}
		if i == 59 {
			t.Fatalf("timed out waiting for %s API server to be ready: %v", distro, err)
		}
		time.Sleep(5 * time.Second)
	}

	// Execute the import command on the init pod with the local kubeconfig and kubectl.
	shell := fmt.Sprintf("export %s && %s", kubectlEnv, importCmd)
	out, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c", shell)
	if err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleEKRError(t, clients, ns.Name, mgmtCluster.Name, err)

	RunEncryptionKeyRotationOperationTest(t, clients, ns.Name, corev1.ObjectReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       mgmtCluster.Name,
	})
}

// This test exercises imported-cluster EKR restart ordering on a mixed RKE2 server topology.
// The topology includes init+etcd, etcd-only, and additional control-plane nodes so restart
// sequencing and final hash convergence are validated through the full imported workflow.
func Test_Operation_SetD_ImportedEncryptionKeyRotation_MultiNodeMixedRoles(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) != "rke2" {
		t.Skip("encryption key rotation")
	}

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
		{ControlPlane: true, ETCD: true, Worker: false, Quantity: 1},  // init + etcd
		{ControlPlane: false, ETCD: true, Worker: false, Quantity: 1}, // etcd-only
		{ControlPlane: true, ETCD: false, Worker: false, Quantity: 1}, // additional control-plane
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
			DisplayName:    "test-imported-encryption-key-rotation-mixed-roles",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleEKRError(t, clients, ns.Name, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	// Wait for the inner API server to be responsive, not just the kubeconfig file.
	for i := 0; i < 60; i++ {
		_, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name,
			"sh", "-c", fmt.Sprintf("export %s && kubectl get nodes", kubectlEnv))
		if err == nil {
			break
		}
		if i == 59 {
			t.Fatalf("timed out waiting for %s API server to be ready: %v", distro, err)
		}
		time.Sleep(5 * time.Second)
	}

	// Execute the import command on the init pod with the local kubeconfig and kubectl.
	shell := fmt.Sprintf("export %s && %s", kubectlEnv, importCmd)
	out, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c", shell)
	if err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleEKRError(t, clients, ns.Name, mgmtCluster.Name, err)

	waitForImportedNodesReady(t, clients, ns.Name, pods[0].Name, kubectlEnv, []string{
		"imported-init-0",
		"imported-node-1",
		"imported-node-2",
	})

	RunEncryptionKeyRotationOperationTest(t, clients, ns.Name, corev1.ObjectReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       mgmtCluster.Name,
	})

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleEKRError(t, clients, ns.Name, mgmtCluster.Name, err)

	waitForImportedNodesReady(t, clients, ns.Name, pods[0].Name, kubectlEnv, []string{
		"imported-init-0",
		"imported-node-1",
		"imported-node-2",
	})

	encStatus, err := cluster.ExecOnPod(
		clients,
		ns.Name,
		pods[len(pods)-1].Name,
		"sh",
		"-c",
		fmt.Sprintf("export PATH=$PATH:%s && rke2 secrets-encrypt status", binDir),
	)
	if err != nil {
		t.Fatalf("failed running rke2 secrets-encrypt status on downstream node: %v", err)
	}
	assert.Contains(t, encStatus, "Current Rotation Stage: reencrypt_finished")
	assert.Contains(t, encStatus, "Server Encryption Hashes: All hashes match")
}

func waitForImportedNodesReady(t *testing.T, clients *clients.Clients, namespace, podName, kubectlEnv string, expectedNodeNames []string) {
	t.Helper()

	var lastOut string
	var lastErr error
	command := fmt.Sprintf("export %s && kubectl get nodes --no-headers 2>/dev/null || true", kubectlEnv)
	for i := 0; i < 180; i++ {
		lastOut, lastErr = cluster.ExecOnPod(clients, namespace, podName, "sh", "-c", command)
		if lastErr == nil && importedNodesReady(lastOut, expectedNodeNames) {
			return
		}
		time.Sleep(5 * time.Second)
	}

	t.Fatalf(
		"timed out waiting for downstream nodes %v to be Ready; last output=%q lastErr=%v",
		expectedNodeNames,
		strings.TrimSpace(lastOut),
		lastErr,
	)
}

func importedNodesReady(output string, expectedNodeNames []string) bool {
	readyNodes := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.HasPrefix(fields[1], "Ready") {
			readyNodes[fields[0]] = true
		}
	}

	for _, nodeName := range expectedNodeNames {
		if !readyNodes[nodeName] {
			return false
		}
	}
	return true
}
