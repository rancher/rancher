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
