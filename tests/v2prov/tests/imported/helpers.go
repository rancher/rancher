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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type importedClusterFixture struct {
	ns          *corev1.Namespace
	pods        []*corev1.Pod
	mgmtCluster *v3.Cluster
	clusterRef  corev1.ObjectReference
	kubectlEnv  string
	execKubectl func(t *testing.T, cmd string) (string, error)
}

func setUpImportedCluster(t *testing.T, clients *clients.Clients, mgmtCluster *v3.Cluster, pools []cluster.ImportedNodePool) *importedClusterFixture {
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

	expectedPods := 0
	for _, pool := range pools {
		expectedPods += pool.Quantity
	}
	if len(pods) != expectedPods {
		t.Fatalf("expected %d imported pod(s), got %d", expectedPods, len(pods))
	}

	mgmtCluster, err = cluster.NewImported(clients, mgmtCluster)
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	if importCmd == "" {
		t.Fatal("import command is empty")
	}

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	execKubectl := func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(
			clients,
			ns.Name,
			pods[0].Name,
			"sh",
			"-c",
			fmt.Sprintf("export %s && %s", kubectlEnv, cmd),
		)
	}

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
		kubectlEnv:  kubectlEnv,
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
