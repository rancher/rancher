package imported

import (
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// importedClusterFixture bundles the resources produced by setUpImportedCluster so each test can
// reach into the imported cluster (via execKubectl) and reference the parent mgmt cluster without
// re-deriving any of it.
type importedClusterFixture struct {
	// ns is the random namespace that holds the simulated node pods.
	ns *corev1.Namespace
	// pods are the simulated nodes, ordered to match the input pool list. pods[0] is the init node.
	pods []*corev1.Pod
	// mgmtCluster is the imported management.cattle.io/v3 Cluster, post-Ready.
	mgmtCluster *v3.Cluster
	// clusterRef is the convenient corev1.ObjectReference used for snapshot save/restore ops.
	clusterRef corev1.ObjectReference
	// execKubectl runs a shell command on the init node with KUBECONFIG/PATH pre-exported so the
	// caller can just write `kubectl ...` without re-deriving the distro paths.
	execKubectl func(t *testing.T, cmd string) (string, error)
}

// setUpImportedCluster brings up an imported cluster end-to-end so a test can move straight to its
// own assertions. It performs the steps that every snapshot save/restore test repeats:
//
//  1. Allocates a random namespace and ensures the shared registry cache.
//  2. Spins up the requested pool topology as pods via cluster.NewImportedClusterPods.
//  3. Creates the management.cattle.io/v3 Cluster and fetches its import command.
//  4. Polls the downstream API server until it responds (the kubeconfig file exists slightly before
//     the API is actually serving — polling avoids racing the registration).
//  5. Executes the import command on the init pod so cattle-cluster-agent registers upward.
//  6. Waits for the mgmt cluster to reach Ready.
//
// On any failure it routes through handleError so the dumped object bundle includes everything we
// might need to triage the bring-up.
func setUpImportedCluster(t *testing.T, clients *clients.Clients, displayName string, pools []cluster.ImportedNodePool) *importedClusterFixture {
	t.Helper()

	ns, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	registryCACert, err := registry.EnsureRegistryCache(clients)
	if err != nil {
		t.Fatal(err)
	}

	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, defaults.SomeK8sVersion, pools, nil, registryCACert)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity-check the pool sum matches what came back; downstream tests index pods[0] as the init
	// node and would fail with a confusing nil deref if the pool/pod counts diverged.
	want := 0
	for _, p := range pools {
		want += p.Quantity
	}
	assert.Len(t, pods, want)

	mgmtCluster, err := cluster.NewImported(clients, &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "c-",
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    displayName,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	// Build the env prefix once — every kubectl invocation inside the imported cluster needs
	// KUBECONFIG and the rke2/k3s binary directory on PATH.
	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	execKubectl := func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
			fmt.Sprintf("export %s && %s", kubectlEnv, cmd))
	}

	// Poll the inner API server. The kubeconfig file appears slightly before the API is actually
	// serving; without this loop the import-command exec races and intermittently fails.
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

	if out, err := execKubectl(t, importCmd); err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleError(t, clients, mgmtCluster.Name, err)

	return &importedClusterFixture{
		ns:          ns,
		pods:        pods,
		mgmtCluster: mgmtCluster,
		clusterRef: corev1.ObjectReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
			Name:       mgmtCluster.Name,
		},
		execKubectl: execKubectl,
	}
}

func handleError(t *testing.T, clients *clients.Clients, name string, err error) {
	if err != nil {
		objs := map[string]any{}

		c, newErr := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["mgmtCluster"] = c
			nodes, newErr := clients.Mgmt.Node().List(c.Name, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["mgmtNodes"] = nodes
			}

			beacon, newErr := clients.Plan.Beacon().Get(c.Name, c.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["beacon"] = beacon
			}

			secrets, newErr := clients.Core.Secret().List(c.Name, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name),
				FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
			})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["machinePlans"] = secrets
			}

			creates, newErr := clients.Operation.ETCDSnapshotSave().List(c.Name, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["ETCDSnapshotSave"] = creates
			}

			restores, newErr := clients.Operation.ETCDSnapshotRestore().List(c.Name, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["ETCDSnapshotRestore"] = restores
			}
		}

		features, newErr := clients.Mgmt.Feature().List(metav1.ListOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["features"] = features
		}

		settings, newErr := clients.Mgmt.Setting().List(metav1.ListOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["settings"] = settings
		}

		data, newErr := snapshotutil.CompressInterface(objs)
		if newErr != nil {
			logrus.Error(newErr)
		}
		//nolint:revive
		err = fmt.Errorf("cluster %s operation wait failed on: %w\ncluster %s test data bundle: \n%s\n", name, err, name, data)
		t.Fatal(err)
	}
}
