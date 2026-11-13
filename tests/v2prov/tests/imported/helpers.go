package imported

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/operations"
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
	// cs is used to exec into kubectlPod.
	cs *clients.Clients
	// kubectlPod is the server node execKubectl runs on. It starts as the init node.
	kubectlPod *corev1.Pod
	// kubectlEnv is the "KUBECONFIG=... PATH=$PATH:..." prefix resolved for the init node's
	// effective runtime binary location, so later helpers (e.g. rotation recovery) don't have to
	// re-derive it and risk assuming the default data directory.
	kubectlEnv string
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
// If the test fails at any point after the mgmt cluster is created, a debug bundle covering both the
// mgmt cluster and the downstream cluster is printed when clients is closed.
func setUpImportedCluster(t *testing.T, clients *clients.Clients, displayName string, pools []cluster.ImportedNodePool) *importedClusterFixture {
	t.Helper()

	return setUpImportedClusterAtVersion(t, clients, displayName, pools, defaults.SomeK8sVersion)
}

// setUpImportedClusterAtVersion is setUpImportedCluster with the distro version pinned, for tests
// that need to bring a cluster up on one version and then move it to another.
//
// opts, when supplied, is passed through to cluster.NewImportedClusterPods — used by the restore-mode
// tests to point the cluster's etcd snapshots at an object store.
func setUpImportedClusterAtVersion(t *testing.T, clients *clients.Clients, displayName string, pools []cluster.ImportedNodePool, k8sVersion string, opts ...cluster.ImportedClusterOptions) *importedClusterFixture {
	t.Helper()

	return setUpImportedClusterWithHook(t, clients, displayName, pools, k8sVersion, nil, opts...)
}

// setUpImportedClusterWithHook is setUpImportedClusterAtVersion with beforeImport, when non-nil,
// called once the downstream API server is serving and before the cluster is imported. The fixture
// it receives has everything but mgmtCluster's final, Ready state. It is for setup the import itself
// depends on — with cluster.ImportedClusterOptions.ExternalCloudProvider, for instance, nodes stay
// tainted until their addresses are published, and the cluster agent cannot schedule until then.
func setUpImportedClusterWithHook(t *testing.T, clients *clients.Clients, displayName string, pools []cluster.ImportedNodePool, k8sVersion string, beforeImport func(fx *importedClusterFixture), opts ...cluster.ImportedClusterOptions) *importedClusterFixture {
	t.Helper()

	ns, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	endPhase := phase(t, "setup/registry")
	registryCACert, err := registry.EnsureRegistryCache(clients)
	if err != nil {
		t.Fatal(err)
	}
	endPhase()

	endPhase = phase(t, "setup/pods")
	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, k8sVersion, pools, nil, registryCACert, opts...)
	if err != nil {
		t.Fatal(err)
	}
	endPhase()

	// Sanity-check the pool sum matches what came back; downstream tests index pods[0] as the init
	// node and would fail with a confusing nil deref if the pool/pod counts diverged.
	want := 0
	for _, p := range pools {
		want += p.Quantity
	}
	if len(pods) != want {
		t.Fatalf("expected %d pod(s) for imported pools, got %d", want, len(pods))
	}

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
	printDebugDataOnFailure(t, clients, mgmtCluster.Name, ns.Name)

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	// Build the env prefix once — every kubectl invocation inside the imported cluster needs
	// KUBECONFIG and the rke2/k3s binary directory on PATH.
	distro := capr.GetRuntime(k8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	var binDir string
	if distro == capr.RuntimeRKE2 {
		binDir = path.Join(initNodeDataDirFromPools(distro, pools), "bin")
	} else {
		binDir = "/usr/local/bin"
	}
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	execKubectl := func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c",
			fmt.Sprintf("export %s && %s", kubectlEnv, cmd))
	}

	// Poll the inner API server. The kubeconfig file appears slightly before the API is actually
	// serving; without this loop the import-command exec races and intermittently fails.
	endPhase = phase(t, "setup/api")
	limit := 120
	for i := 0; i < limit; i++ {
		_, err := execKubectl(t, "kubectl get nodes")
		if err == nil {
			break
		}
		if i == limit-1 {
			t.Fatalf("timed out waiting for %s API server to be ready: %v", distro, err)
		}
		time.Sleep(5 * time.Second)
	}
	endPhase()

	fx := &importedClusterFixture{
		ns:          ns,
		pods:        pods,
		mgmtCluster: mgmtCluster,
		clusterRef: corev1.ObjectReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
			Name:       mgmtCluster.Name,
		},
		cs:         clients,
		kubectlPod: pods[0],
		kubectlEnv: kubectlEnv,
	}
	if beforeImport != nil {
		endPhase = phase(t, "setup/before-import")
		beforeImport(fx)
		endPhase()
	}

	endPhase = phase(t, "setup/import")
	if out, err := execKubectl(t, importCmd); err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.ClusterConditionReady.IsTrue(mgmtCluster), nil
	})
	handleError(t, clients, mgmtCluster.Name, err)
	endPhase()

	fx.mgmtCluster = mgmtCluster
	return fx
}

// kubectlCallTimeout bounds each kubectl call made through kubectl, so a hung exec can't eat the caller's deadline.
const kubectlCallTimeout = 30 * time.Second

// kubectl adapts execKubectl to operations.Kubectl, for helpers that take one. This allows
// us to run the same helper against a cluster reachable only via kubectl on one of its nodes, or against a
// cluster reachable directly via the mgmt cluster's kubeconfig.
func (fx *importedClusterFixture) kubectl(t *testing.T) operations.Kubectl {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(ctx, kubectlCallTimeout)
		defer cancel()

		quoted := make([]string, len(args))
		for i, arg := range args {
			quoted[i] = shellQuote(arg)
		}
		out, err := fx.execKubectlContext(ctx, "kubectl "+strings.Join(quoted, " "))
		return []byte(out), err
	}
}

// execKubectl runs a shell command on kubectlPod with KUBECONFIG/PATH pre-exported so the caller can
// just write `kubectl ...` without re-deriving the distro paths.
func (fx *importedClusterFixture) execKubectl(t *testing.T, cmd string) (string, error) {
	t.Helper()
	return fx.execKubectlContext(fx.cs.Ctx, cmd)
}

// execKubectlContext is execKubectl, stopping the exec when ctx is done.
func (fx *importedClusterFixture) execKubectlContext(ctx context.Context, cmd string) (string, error) {
	return cluster.ExecOnPodContext(ctx, fx.cs, fx.ns.Name, fx.kubectlPod.Name, "sh", "-c",
		fmt.Sprintf("export %s && %s", fx.kubectlEnv, cmd))
}

// shellQuote wraps value in single quotes so it is passed through `sh -c` as one literal argument.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// phase logs a start marker for the named phase and returns a func that logs its end. The markers
// carry UTC timestamps so a run's phases can be lined up with host telemetry. A phase with a start
// and no end was in progress when the test failed.
func phase(t *testing.T, name string) func() {
	t.Helper()

	start := time.Now()
	t.Logf("PHASE %s start %s", name, start.UTC().Format(time.RFC3339Nano))
	return func() {
		t.Helper()
		end := time.Now()
		t.Logf("PHASE %s end %s elapsed=%s", name, end.UTC().Format(time.RFC3339Nano), end.Sub(start).Round(time.Millisecond))
	}
}

// initNodeDataDirFromPools returns the configured data directory of the bootstrap etcd pool (the
// first pool with ETCD: true and Quantity > 0, matching cluster.NewImportedClusterPods' init-node
// selection), or the runtime default when that pool sets none.
func initNodeDataDirFromPools(distro string, pools []cluster.ImportedNodePool) string {
	for _, pool := range pools {
		if pool.ETCD && pool.Quantity > 0 {
			if pool.DistroDataDir != "" {
				return pool.DistroDataDir
			}
			break
		}
	}
	return fmt.Sprintf("/var/lib/rancher/%s", distro)
}

func handleError(t *testing.T, clients *clients.Clients, name string, err error) {
	if err != nil {
		//nolint:revive
		err = fmt.Errorf("cluster %s operation wait failed on: %w\ncluster %s test data bundle: \n%s\n", name, err, name, gatherDebugData(clients, name, ""))
		t.Fatal(err)
	}
}

// printDebugDataOnFailure registers a hook on clients that prints a debug bundle for the imported cluster name if t has
// failed by the time clients is closed, including the downstream cluster's view gathered from the node pods in
// nodeNamespace. It runs ahead of the hooks registered before it, so register it after the node namespace is created to
// gather from it before it is deleted.
func printDebugDataOnFailure(t *testing.T, clients *clients.Clients, name, nodeNamespace string) {
	clients.OnClose(func() {
		if !t.Failed() {
			return
		}
		fmt.Printf("cluster %s test failed\ncluster %s test data bundle: \n%s\n", name, name, gatherDebugData(clients, name, nodeNamespace))
	})
}

// gatherDebugData returns a gzip compressed + base64 encoded json bundle of the mgmt-side state of the imported cluster
// name. When nodeNamespace is set it also includes the downstream cluster's pods and cattle-cluster-agent logs, gathered
// from the imported node pods in nodeNamespace.
func gatherDebugData(clients *clients.Clients, name, nodeNamespace string) string {
	objs := map[string]any{}

	if nodeNamespace != "" {
		objs["downstream"] = cluster.GatherImportedDownstreamData(clients, nodeNamespace)
	}

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
	return data
}
