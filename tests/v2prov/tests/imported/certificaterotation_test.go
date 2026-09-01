package imported

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/operations/certificaterotation"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

type certificateMetadata struct {
	path              string
	serial            string
	notBeforeRaw      string
	notAfterRaw       string
	notBefore         time.Time
	notAfter          time.Time
	fingerprintSHA256 string
}

type nodeCertificateMetadata struct {
	podName string
	records map[string]certificateMetadata
}

// Test_Imported_Operation_SetD_ImportedCertificateRotation validates the baseline single-node
// imported flow: certificate rotation succeeds and every server certificate is replaced.
func Test_Imported_Operation_SetD_ImportedCertificateRotation(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Setup: bring up a single-node imported cluster with the default runtime data directory.
	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// Before-rotation evidence: record certificate metadata so rotation can be proven later.
	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	requiredPaths := requiredCertificatePaths(runtimeName)
	before := collectRequiredCertificateMetadata(t, fx, requiredPaths)

	// Operation execution: run CertificateRotation to completion.
	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assertCertificateRotationSucceeded(t, op)
	op = WaitForCertificateRotationSucceeded(t, cs, op, fx.mgmtCluster.Name, fx.mgmtCluster.Name)
	assertCertificateRotationSucceeded(t, op)

	// Recovery: the cluster must come back healthy on the rotated certificates.
	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)

	// Final assertions: every required certificate actually rotated, and the API still works.
	after := collectRequiredCertificateMetadata(t, fx, requiredPaths)
	assertCertificateRotationMetadata(t, before, after, requiredPaths)

	assertDownstreamAPIUsableAfterRotation(t, fx)
}

// Test_Imported_Operation_SetD_ImportedCertificateRotation_Custom_Data_Dir validates that
// certificate rotation on an imported node correctly resolves and rotates certificates from a
// custom RKE2/K3s data directory configured before bootstrap, rather than the runtime default.
func Test_Imported_Operation_SetD_ImportedCertificateRotation_Custom_Data_Dir(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Setup: bring up a single-node imported cluster with a non-default data-dir configured
	// before bootstrap.
	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	dataDir := fmt.Sprintf("/var/lib/rancher/testing/certificate-rotation-%s", runtimeName)

	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-custom-data-dir", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1, DistroDataDir: dataDir},
	})

	// The node must actually report the custom data-dir before trusting cert paths derived from it.
	assertNodeArgsContainDataDir(t, cs, fx.mgmtCluster.Name, runtimeName, dataDir)

	// Before-rotation evidence, using cert paths under the custom data-dir.
	requiredPaths := requiredCertificatePathsForDataDir(dataDir)
	before := collectRequiredCertificateMetadata(t, fx, requiredPaths)

	// Operation execution.
	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assertCertificateRotationSucceeded(t, op)
	op = WaitForCertificateRotationSucceeded(t, cs, op, fx.mgmtCluster.Name, fx.mgmtCluster.Name)
	assertCertificateRotationSucceeded(t, op)

	// Recovery.
	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)
	assertDownstreamAPIUsableAfterRotation(t, fx)

	// Final assertions: certificates under the custom data-dir actually rotated.
	after := collectRequiredCertificateMetadata(t, fx, requiredPaths)
	assertCertificateRotationMetadata(t, before, after, requiredPaths)
}

// Test_Imported_Operation_SetD_ImportedCertificateRotation_Service_Argument validates that
// restricting rotation to the "etcd" service rotates only etcd certificates and leaves the rest
// of the server certificates untouched.
func Test_Imported_Operation_SetD_ImportedCertificateRotation_Service_Argument(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Setup.
	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-service-argument", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// Before-rotation evidence, split by whether the cert is expected to rotate.
	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	tlsDir := fmt.Sprintf("/var/lib/rancher/%s/server/tls", runtimeName)
	etcdPaths := []string{
		path.Join(tlsDir, "etcd/server-client.crt"),
		path.Join(tlsDir, "etcd/peer-server-client.crt"),
	}
	nonEtcdPaths := []string{
		path.Join(tlsDir, "client-admin.crt"),
		path.Join(tlsDir, "serving-kube-apiserver.crt"),
		path.Join(tlsDir, "kube-controller-manager/kube-controller-manager.crt"),
		path.Join(tlsDir, "kube-scheduler/kube-scheduler.crt"),
	}
	allPaths := append(append([]string(nil), etcdPaths...), nonEtcdPaths...)
	before := collectRequiredCertificateMetadata(t, fx, allPaths)

	// Operation execution, scoped to the etcd service only.
	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef, WithCertificateRotationServices("etcd"))
	assert.Equal(t, []string{"etcd"}, op.Spec.Args.Services)
	assertCertificateRotationSucceeded(t, op)

	// Recovery.
	op = WaitForCertificateRotationSucceeded(t, cs, op, fx.mgmtCluster.Name, fx.mgmtCluster.Name)
	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)
	assertDownstreamAPIUsableAfterRotation(t, fx)

	// Final assertions: only etcd certs rotated, everything else stayed the same.
	after := collectRequiredCertificateMetadata(t, fx, allPaths)
	assertCertificateRotationMetadata(t, before, after, etcdPaths)
	assertCertificateMetadataUnchanged(t, before, after, nonEtcdPaths)
}

// Test_Imported_Operation_SetD_ImportedCertificateRotation_RKE2_TLS_Args validates that when the
// kube-controller-manager/kube-scheduler serve on a custom TLS cert/key pair (via RKE2 node
// args), CertificateRotation reads those node args, preserves the custom TLS pair, and rotates
// the remaining targeted server certificates.
func Test_Imported_Operation_SetD_ImportedCertificateRotation_RKE2_TLS_Args(t *testing.T) {
	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	if runtimeName != capr.RuntimeRKE2 {
		t.Skip("RKE2-only: validates imported RKE2 component TLS node arguments")
	}

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Setup: bring up a single-node imported RKE2 cluster.
	dataDir := "/var/lib/rancher/rke2"
	kcmDefaultCert := dataDir + "/server/tls/kube-controller-manager/kube-controller-manager.crt"
	kcmDefaultKey := dataDir + "/server/tls/kube-controller-manager/kube-controller-manager.key"
	ksDefaultCert := dataDir + "/server/tls/kube-scheduler/kube-scheduler.crt"
	ksDefaultKey := dataDir + "/server/tls/kube-scheduler/kube-scheduler.key"
	kcmCustomDir := dataDir + "/server/tls/rotation-test/kube-controller-manager"
	ksCustomDir := dataDir + "/server/tls/rotation-test/kube-scheduler"
	kcmCustomCert := kcmCustomDir + "/kube-controller-manager.crt"
	kcmCustomKey := kcmCustomDir + "/kube-controller-manager.key"
	ksCustomCert := ksCustomDir + "/kube-scheduler.crt"
	ksCustomKey := ksCustomDir + "/kube-scheduler.key"
	configPath := "/etc/rancher/rke2/config.yaml.d/60-certificate-rotation-tls-test.yaml"

	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-rke2-tls-args", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// Reconfigure RKE2 to serve kube-controller-manager/kube-scheduler on a custom cert/key pair,
	// via node args, so rotation must read node args rather than assuming default cert paths.
	configureCommand := strings.Join([]string{
		"set -e",
		fmt.Sprintf("mkdir -p %s %s", shellQuote(kcmCustomDir), shellQuote(ksCustomDir)),
		fmt.Sprintf("cp %s %s", shellQuote(kcmDefaultCert), shellQuote(kcmCustomCert)),
		fmt.Sprintf("cp %s %s", shellQuote(kcmDefaultKey), shellQuote(kcmCustomKey)),
		fmt.Sprintf("cp %s %s", shellQuote(ksDefaultCert), shellQuote(ksCustomCert)),
		fmt.Sprintf("cp %s %s", shellQuote(ksDefaultKey), shellQuote(ksCustomKey)),
		fmt.Sprintf("cat > %s <<'EOF'\nkube-controller-manager-arg:\n  - secure-port=10261\n  - tls-cert-file=%s\n  - tls-private-key-file=%s\nkube-scheduler-arg:\n  - secure-port=10262\n  - tls-cert-file=%s\n  - tls-private-key-file=%s\nEOF", shellQuote(configPath), kcmCustomCert, kcmCustomKey, ksCustomCert, ksCustomKey),
		"systemctl restart rke2-server",
	}, "\n")
	if out, err := cluster.ExecOnPod(cs, fx.ns.Name, fx.pods[0].Name, "sh", "-c", configureCommand); err != nil {
		t.Fatalf("failed to configure RKE2 TLS arguments on pod %s: %v\noutput: %s", fx.pods[0].Name, err, strings.TrimSpace(out))
	}

	// Reconfiguration verification: wait for RKE2 and Rancher to recover, and confirm the
	// registered node publishes the TLS arguments that rotation must later read.
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, fx.kubectlEnv, []string{"imported-init-0"})
	if out, err := fx.execKubectl(t, "kubectl get --raw=/readyz"); err != nil {
		t.Fatalf("downstream API did not become ready after RKE2 TLS reconfiguration: %v\noutput: %s", err, strings.TrimSpace(out))
	}

	expectedNodeArgs := []string{
		"secure-port=10261",
		"tls-cert-file=" + kcmCustomCert,
		"tls-private-key-file=" + kcmCustomKey,
		"secure-port=10262",
		"tls-cert-file=" + ksCustomCert,
		"tls-private-key-file=" + ksCustomKey,
	}
	missingNodeArgs := append([]string(nil), expectedNodeArgs...)
	err = utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		nodes, err := cs.Mgmt.Node().List(fx.mgmtCluster.Name, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		missingNodeArgs = append(missingNodeArgs[:0], expectedNodeArgs...)
		if len(nodes.Items) == 1 {
			annotation := nodes.Items[0].Status.NodeAnnotations["rke2.io/node-args"]
			for i := 0; i < len(missingNodeArgs); {
				if strings.Contains(annotation, missingNodeArgs[i]) {
					missingNodeArgs = append(missingNodeArgs[:i], missingNodeArgs[i+1:]...)
					continue
				}
				i++
			}
		}
		return len(missingNodeArgs) == 0, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for RKE2 node args to contain expected TLS values; missing values: %q: %v", missingNodeArgs, err)
	}

	mgmtCluster, err := cs.Mgmt.Cluster().Get(fx.mgmtCluster.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get imported management cluster %s after RKE2 TLS reconfiguration: %v", fx.mgmtCluster.Name, err)
	}
	err = wait.ClusterObject(cs.Ctx, cs.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for imported management cluster %s to be Ready after RKE2 TLS reconfiguration: %v", fx.mgmtCluster.Name, err)
	}

	// Before-rotation evidence, split by whether the cert is expected to rotate: the custom
	// kube-controller-manager/kube-scheduler pair is the active serving cert and must be left alone.
	rotatingPaths := []string{
		dataDir + "/server/tls/client-admin.crt",
		dataDir + "/server/tls/serving-kube-apiserver.crt",
		dataDir + "/server/tls/etcd/server-client.crt",
		dataDir + "/server/tls/etcd/peer-server-client.crt",
	}
	unchangedPaths := []string{
		kcmCustomCert,
		ksCustomCert,
	}
	allPaths := append(append([]string(nil), rotatingPaths...), unchangedPaths...)
	before := collectRequiredCertificateMetadata(t, fx, allPaths)

	// Operation execution.
	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assertCertificateRotationSucceeded(t, op)
	op = WaitForCertificateRotationSucceeded(t, cs, op, fx.mgmtCluster.Name, fx.mgmtCluster.Name)

	// Recovery.
	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)
	assertDownstreamAPIUsableAfterRotation(t, fx)

	// Final assertions: server certs rotated, the custom TLS pair did not.
	after := collectRequiredCertificateMetadata(t, fx, allPaths)
	assertCertificateRotationMetadata(t, before, after, rotatingPaths)
	assertCertificateMetadataUnchanged(t, before, after, unchangedPaths)
}

// shellQuote wraps value in single quotes so it is passed through `sh -c` as one literal argument.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Test_Imported_Operation_SetD_ImportedCertificateRotationLifecycleHook validates that a
// delegate registered on the Rotate step hook and the Succeeded phase hook actually pauses the
// operation at each hook point, and that the operation still completes and rotates once each
// hook is advanced.
func Test_Imported_Operation_SetD_ImportedCertificateRotationLifecycleHook(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Setup.
	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-lifecycle-hook", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// Before-rotation evidence.
	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	requiredPaths := requiredCertificatePaths(runtimeName)
	before := collectRequiredCertificateMetadata(t, fx, requiredPaths)

	const (
		hookName     = "v2prov-e2e-test"
		delegateName = "v2prov-e2e-test-delegate"
	)
	rotateHookKey := certificaterotation.RotateStepHookLabelPrefix + hookName
	succeededHookKey := planv1alpha1.SucceededPhaseHookLabelPrefix + hookName

	// Operation execution, delegated at both the Rotate step hook and the Succeeded phase hook.
	op := CreateCertificateRotationOp(t, cs, fx.ns.Name, fx.clusterRef, WithCertificateRotationLabels(map[string]string{
		rotateHookKey:    delegateName,
		succeededHookKey: delegateName,
	}))

	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	cp := WaitForCertificateRotationHookPause(t, cs, op, beaconNS, beaconName, rotateHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.CertificateRotationStepRotate)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, cp.Op.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, cp.Op.Status.Step)
	assert.True(t, plan.IsInDelegateChain(cp.Beacon, delegateName), "delegate %q not present in beacon chain at Rotate hook", delegateName)
	AdvancePastCertificateRotationHook(t, cs, op, beaconNS, beaconName, rotateHookKey, delegateName)

	cp = WaitForCertificateRotationHookPause(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName,
		opv1alpha1.OperationPhaseSucceeded, "")
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, cp.Op.Status.Phase)
	assert.True(t, plan.IsInDelegateChain(cp.Beacon, delegateName), "delegate %q not present in beacon chain at Succeeded hook", delegateName)
	AdvancePastCertificateRotationHook(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName)

	final := WaitForCertificateRotationSucceeded(t, cs, op, beaconNS, beaconName)
	assertCertificateRotationSucceeded(t, final)

	// Recovery.
	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, final)

	// Final assertions: rotation still happened despite pausing at both hooks.
	after := collectRequiredCertificateMetadata(t, fx, requiredPaths)
	assertCertificateRotationMetadata(t, before, after, requiredPaths)
}

// Test_Imported_Operation_SetD_ImportedCertificateRotation_Multi_Node validates a lightweight
// role-separated imported topology (1 etcd-only, 1 control-plane-only, 2 worker-only). It
// confirms server certificate rotation happened on etcd/control-plane nodes and that worker-only
// runtime-agent restart paths actually executed. This is not an HA scale test.
func Test_Imported_Operation_SetD_ImportedCertificateRotation_Multi_Node(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Setup: 1 etcd-only, 1 control-plane-only, 2 worker-only nodes.
	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-multi-node", []cluster.ImportedNodePool{
		{ETCD: true, Quantity: 1},
		{ControlPlane: true, Quantity: 1},
		{Worker: true, Quantity: 2},
	})

	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	expectedNodes := []string{
		"imported-init-0",
		"imported-node-1",
		"imported-node-2",
		"imported-node-3",
	}
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, fx.kubectlEnv, expectedNodes)

	etcdCertPaths := []string{
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/etcd/server-client.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/etcd/peer-server-client.crt", runtimeName),
	}
	controlPlaneCertPaths := []string{
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/client-admin.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/serving-kube-apiserver.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/kube-controller-manager/kube-controller-manager.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/kube-scheduler/kube-scheduler.crt", runtimeName),
	}

	etcdPodNames := []string{fx.pods[0].Name}
	controlPlanePodNames := []string{fx.pods[1].Name}
	workerPodNames := []string{fx.pods[2].Name, fx.pods[3].Name}

	// Before-rotation evidence, per role: server certs for etcd/control-plane, agent restart
	// timestamp for workers (workers have no server certs to rotate).
	beforeEtcd := collectNodeCertificateMetadata(t, cs, fx, etcdPodNames, etcdCertPaths)
	beforeControlPlane := collectNodeCertificateMetadata(t, cs, fx, controlPlanePodNames, controlPlaneCertPaths)
	beforeWorkerAgentTimestamp := collectWorkerAgentActiveTimestamp(t, cs, fx, workerPodNames, runtimeName)

	// Operation execution.
	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assertCertificateRotationSucceeded(t, op)

	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name
	op = WaitForCertificateRotationSucceeded(t, cs, op, beaconNS, beaconName)

	// Recovery.
	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, fx.kubectlEnv, expectedNodes)
	assertDownstreamAPIUsableAfterRotation(t, fx)

	// Final assertions: etcd/control-plane certs rotated, worker agents restarted.
	afterEtcd := collectNodeCertificateMetadata(t, cs, fx, etcdPodNames, etcdCertPaths)
	afterControlPlane := collectNodeCertificateMetadata(t, cs, fx, controlPlanePodNames, controlPlaneCertPaths)
	afterWorkerAgentTimestamp := collectWorkerAgentActiveTimestamp(t, cs, fx, workerPodNames, runtimeName)

	assertNodeCertificateRotationMetadata(t, beforeEtcd, afterEtcd, etcdCertPaths)
	assertNodeCertificateRotationMetadata(t, beforeControlPlane, afterControlPlane, controlPlaneCertPaths)
	assertWorkerAgentRestarted(t, beforeWorkerAgentTimestamp, afterWorkerAgentTimestamp)
}

// assertNodeArgsContainDataDir waits for the registered management-cluster Node to publish a
// runtime node-args annotation whose parsed arguments include the expected custom data-dir. Node
// registration is eventually consistent, so this polls rather than checking once.
func assertNodeArgsContainDataDir(t *testing.T, cs *clients.Clients, mgmtClusterName, runtimeName, dataDir string) {
	t.Helper()

	annotationKey := "rke2.io/node-args"
	if runtimeName == capr.RuntimeK3S {
		annotationKey = "k3s.io/node-args"
	}

	var nodeName string
	var lastErr error
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		nodes, err := cs.Mgmt.Node().List(mgmtClusterName, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if len(nodes.Items) == 0 {
			lastErr = fmt.Errorf("no management-cluster Nodes registered")
			return false, nil
		}
		for _, node := range nodes.Items {
			nodeName = node.Name
			if lastErr = nodeArgsContainDataDir(node.Status.NodeAnnotations, annotationKey, dataDir); lastErr == nil {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("node-args annotation did not expose expected data-dir: runtime=%s annotation=%s expectedDataDir=%s mgmtNode=%s: %v",
			runtimeName, annotationKey, dataDir, nodeName, lastErr)
	}
}

// nodeArgsContainDataDir parses the given runtime node-args annotation as a JSON []string and
// checks it for the expected custom data-dir in any valid CLI form.
func nodeArgsContainDataDir(annotations map[string]string, annotationKey, dataDir string) error {
	raw, ok := annotations[annotationKey]
	if !ok || raw == "" {
		return fmt.Errorf("missing %s annotation", annotationKey)
	}

	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return fmt.Errorf("failed to parse %s annotation JSON: %w", annotationKey, err)
	}

	for i, arg := range args {
		switch {
		case (arg == "--data-dir" || arg == "-d") && i+1 < len(args) && args[i+1] == dataDir:
			return nil
		case arg == "--data-dir="+dataDir, arg == "-d="+dataDir:
			return nil
		}
	}
	return fmt.Errorf("expected data-dir argument %q not found in %s", dataDir, annotationKey)
}

// requiredCertificatePaths returns the server certificate paths rotation must replace, under the
// runtime's default data directory.
func requiredCertificatePaths(runtimeName string) []string {
	return requiredCertificatePathsForDataDir(fmt.Sprintf("/var/lib/rancher/%s", runtimeName))
}

// requiredCertificatePathsForDataDir returns the server certificate paths rotation must replace,
// under the given data directory.
func requiredCertificatePathsForDataDir(dataDir string) []string {
	return []string{
		path.Join(dataDir, "server/tls/client-admin.crt"),
		path.Join(dataDir, "server/tls/serving-kube-apiserver.crt"),
		path.Join(dataDir, "server/tls/etcd/server-client.crt"),
		path.Join(dataDir, "server/tls/etcd/peer-server-client.crt"),
		path.Join(dataDir, "server/tls/kube-controller-manager/kube-controller-manager.crt"),
		path.Join(dataDir, "server/tls/kube-scheduler/kube-scheduler.crt"),
	}
}

// collectRequiredCertificateMetadata collects certificate metadata from the init pod for each
// path, keyed by path.
func collectRequiredCertificateMetadata(t *testing.T, fx *importedClusterFixture, paths []string) map[string]certificateMetadata {
	t.Helper()

	metadata := make(map[string]certificateMetadata, len(paths))
	for _, certPath := range paths {
		metadata[certPath] = collectCertificateMetadata(t, fx, certPath)
	}
	return metadata
}

// collectCertificateMetadata reads and parses one certificate's metadata from the init pod.
func collectCertificateMetadata(t *testing.T, fx *importedClusterFixture, certPath string) certificateMetadata {
	t.Helper()

	out, err := execCertificateMetadataCommandOnInitPod(t, fx, certPath)
	if err != nil {
		t.Fatalf("failed collecting metadata for certificate %s: %v\noutput: %s", certPath, err, strings.TrimSpace(out))
	}

	record, err := parseCertificateMetadataOutput(certPath, out)
	if err != nil {
		t.Fatalf("failed parsing certificate metadata for %s: %v\nraw=%q", certPath, err, strings.TrimSpace(out))
	}
	return record
}

// execCertificateMetadataCommandOnInitPod runs the openssl metadata command on the init pod via
// fx.execKubectl.
func execCertificateMetadataCommandOnInitPod(t *testing.T, fx *importedClusterFixture, certPath string) (string, error) {
	t.Helper()

	cmd := buildCertificateMetadataCommand(certPath)
	return fx.execKubectl(t, cmd)
}

// collectCertificateMetadataFromPod reads and parses one certificate's metadata from a specific
// pod, used when the pod under test isn't the fixture's init pod.
func collectCertificateMetadataFromPod(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, podName, certPath string) certificateMetadata {
	t.Helper()

	cmd := buildCertificateMetadataCommand(certPath)
	out, err := cluster.ExecOnPod(clients, fx.ns.Name, podName, "sh", "-c", cmd)
	if err != nil {
		t.Fatalf("failed collecting certificate metadata on pod %s for %s: %v\noutput: %s", podName, certPath, err, strings.TrimSpace(out))
	}

	record, err := parseCertificateMetadataOutput(certPath, out)
	if err != nil {
		t.Fatalf("failed parsing certificate metadata on pod %s for %s: %v\nraw=%q", podName, certPath, err, strings.TrimSpace(out))
	}
	return record
}

// buildCertificateMetadataCommand builds a shell command that fails loudly if certPath is
// missing, otherwise prints its serial, validity window, and SHA-256 fingerprint via openssl.
func buildCertificateMetadataCommand(certPath string) string {
	return fmt.Sprintf(
		"if [ ! -f %q ]; then echo \"missing certificate: %s\" >&2; exit 1; fi; openssl x509 -in %q -noout -serial -startdate -enddate -fingerprint -sha256",
		certPath, certPath, certPath,
	)
}

// parseCertificateMetadataOutput parses buildCertificateMetadataCommand's openssl output into a
// certificateMetadata record.
func parseCertificateMetadataOutput(certPath, out string) (certificateMetadata, error) {
	record := certificateMetadata{path: certPath}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "serial="):
			record.serial = strings.TrimPrefix(line, "serial=")
		case strings.HasPrefix(line, "notBefore="):
			record.notBeforeRaw = strings.TrimPrefix(line, "notBefore=")
		case strings.HasPrefix(line, "notAfter="):
			record.notAfterRaw = strings.TrimPrefix(line, "notAfter=")
		case strings.Contains(line, "Fingerprint="):
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				record.fingerprintSHA256 = strings.TrimSpace(parts[1])
			}
		}
	}

	if record.serial == "" || record.notBeforeRaw == "" || record.notAfterRaw == "" || record.fingerprintSHA256 == "" {
		return record, fmt.Errorf("incomplete metadata: serial=%q notBefore=%q notAfter=%q fingerprint=%q",
			record.serial, record.notBeforeRaw, record.notAfterRaw, record.fingerprintSHA256)
	}

	notBefore, err := time.Parse("Jan _2 15:04:05 2006 MST", record.notBeforeRaw)
	if err != nil {
		return record, fmt.Errorf("parse notBefore %q: %w", record.notBeforeRaw, err)
	}
	notAfter, err := time.Parse("Jan _2 15:04:05 2006 MST", record.notAfterRaw)
	if err != nil {
		return record, fmt.Errorf("parse notAfter %q: %w", record.notAfterRaw, err)
	}
	record.notBefore = notBefore
	record.notAfter = notAfter
	return record, nil
}

// assertCertificateRotationSucceeded verifies the operation finished in the Succeeded phase,
// completed the Rotate step, and did not get stuck waiting for its plan to apply.
func assertCertificateRotationSucceeded(t *testing.T, op *opv1alpha1.CertificateRotation) {
	t.Helper()

	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, op.Status.Step)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&op.Status))
}

// assertCertificateRotationMetadata fails the test unless every path in requiredPaths shows a
// changed serial and fingerprint, with notBefore not moving backwards.
func assertCertificateRotationMetadata(t *testing.T, before, after map[string]certificateMetadata, requiredPaths []string) {
	t.Helper()

	for _, certPath := range requiredPaths {
		beforeMeta, ok := before[certPath]
		if !ok {
			t.Fatalf("missing pre-rotation certificate metadata for %s", certPath)
		}
		afterMeta, ok := after[certPath]
		if !ok {
			t.Fatalf("missing post-rotation certificate metadata for %s", certPath)
		}

		detail := fmt.Sprintf(
			"path=%s before(serial=%s fingerprint=%s notBefore=%s notAfter=%s) after(serial=%s fingerprint=%s notBefore=%s notAfter=%s)",
			certPath,
			beforeMeta.serial, beforeMeta.fingerprintSHA256, beforeMeta.notBeforeRaw, beforeMeta.notAfterRaw,
			afterMeta.serial, afterMeta.fingerprintSHA256, afterMeta.notBeforeRaw, afterMeta.notAfterRaw,
		)

		if beforeMeta.fingerprintSHA256 == afterMeta.fingerprintSHA256 {
			t.Fatalf("certificate fingerprint did not change after rotation: %s", detail)
		}
		if beforeMeta.serial == afterMeta.serial {
			t.Fatalf("certificate serial did not change after rotation: %s", detail)
		}
		if afterMeta.notBefore.Before(beforeMeta.notBefore) {
			t.Fatalf("certificate notBefore moved backwards after rotation: %s", detail)
		}
	}
}

// assertCertificateMetadataUnchanged fails the test if any path in paths shows a different
// serial, fingerprint, or validity window between before and after.
func assertCertificateMetadataUnchanged(t *testing.T, before, after map[string]certificateMetadata, paths []string) {
	t.Helper()

	for _, certPath := range paths {
		beforeMeta, ok := before[certPath]
		if !ok {
			t.Fatalf("missing pre-rotation certificate metadata for unchanged path %s", certPath)
		}
		afterMeta, ok := after[certPath]
		if !ok {
			t.Fatalf("missing post-rotation certificate metadata for unchanged path %s", certPath)
		}

		detail := fmt.Sprintf("path=%s before=%+v after=%+v", certPath, beforeMeta, afterMeta)
		if beforeMeta.serial != afterMeta.serial {
			t.Fatalf("certificate serial changed unexpectedly: %s", detail)
		}
		if beforeMeta.fingerprintSHA256 != afterMeta.fingerprintSHA256 {
			t.Fatalf("certificate SHA-256 fingerprint changed unexpectedly: %s", detail)
		}
		if !beforeMeta.notBefore.Equal(afterMeta.notBefore) {
			t.Fatalf("certificate notBefore changed unexpectedly: %s", detail)
		}
		if !beforeMeta.notAfter.Equal(afterMeta.notAfter) {
			t.Fatalf("certificate notAfter changed unexpectedly: %s", detail)
		}
	}
}

// waitForImportedCertificateRotationRecovery waits for the mgmt cluster to go Ready, the init
// node to rejoin, the downstream API to answer, and the runtime's own certificate check to pass.
// On failure it routes through handleCertificateRotationError so the dumped bundle covers the op.
func waitForImportedCertificateRotationRecovery(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, runtimeName string, op *opv1alpha1.CertificateRotation) {
	t.Helper()

	mgmtCluster := fx.mgmtCluster
	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleCertificateRotationError(t, clients, fx.mgmtCluster.Name, op, err)

	waitForImportedNodesReady(t, clients, fx.ns.Name, fx.pods[0].Name, fx.kubectlEnv, []string{"imported-init-0"})

	if out, err := fx.execKubectl(t, "kubectl get --raw=/readyz"); err != nil {
		t.Fatalf("downstream API readyz check failed: %v\noutput: %s", err, strings.TrimSpace(out))
	}

	out, err := fx.execKubectl(t, fmt.Sprintf("%s certificate check --output table", runtimeName))
	if err != nil {
		t.Fatalf("runtime certificate check failed for %s: %v\noutput: %s", runtimeName, err, strings.TrimSpace(out))
	}
}

// assertDownstreamAPIUsableAfterRotation proves the downstream API still serves authenticated
// writes and reads after rotation by round-tripping a throwaway ConfigMap.
func assertDownstreamAPIUsableAfterRotation(t *testing.T, fx *importedClusterFixture) {
	t.Helper()

	cmName := "test-cert-rotation-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10))
	const cmValue = "wow"

	if out, err := fx.execKubectl(t, fmt.Sprintf("kubectl create configmap %s --from-literal=test=%s", cmName, cmValue)); err != nil {
		t.Fatalf("failed creating post-rotation ConfigMap %s: %v\noutput: %s", cmName, err, strings.TrimSpace(out))
	}

	out, err := fx.execKubectl(t, fmt.Sprintf("kubectl get configmap %s -o jsonpath='{.data.test}'", cmName))
	if err != nil {
		t.Fatalf("failed reading post-rotation ConfigMap %s: %v\noutput: %s", cmName, err, strings.TrimSpace(out))
	}
	if got := strings.TrimSpace(out); got != cmValue {
		t.Fatalf("unexpected value in post-rotation ConfigMap %s: expected %q, got %q", cmName, cmValue, got)
	}
}

// collectNodeCertificateMetadata collects certificate metadata for certPaths on each named pod,
// used by the multi-node test where the pod under test differs per role.
func collectNodeCertificateMetadata(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, podNames, certPaths []string) []nodeCertificateMetadata {
	t.Helper()

	nodes := make([]nodeCertificateMetadata, 0, len(podNames))
	for _, podName := range podNames {
		records := make(map[string]certificateMetadata, len(certPaths))
		for _, certPath := range certPaths {
			records[certPath] = collectCertificateMetadataFromPod(t, clients, fx, podName, certPath)
		}
		nodes = append(nodes, nodeCertificateMetadata{
			podName: podName,
			records: records,
		})
	}
	return nodes
}

// assertNodeCertificateRotationMetadata is the per-pod equivalent of
// assertCertificateRotationMetadata: it fails the test unless every required path on every pod
// shows a changed serial and fingerprint, with notBefore not moving backwards.
func assertNodeCertificateRotationMetadata(t *testing.T, before, after []nodeCertificateMetadata, requiredPaths []string) {
	t.Helper()

	if len(before) != len(after) {
		t.Fatalf("mismatched node evidence size: before=%d after=%d", len(before), len(after))
	}

	beforeByPod := map[string]map[string]certificateMetadata{}
	for i := range before {
		beforeByPod[before[i].podName] = before[i].records
	}

	for _, afterNode := range after {
		beforeRecords, ok := beforeByPod[afterNode.podName]
		if !ok {
			t.Fatalf("missing pre-rotation evidence for pod %s", afterNode.podName)
		}
		for _, certPath := range requiredPaths {
			beforeMeta, ok := beforeRecords[certPath]
			if !ok {
				t.Fatalf("missing pre-rotation certificate metadata for pod %s path %s", afterNode.podName, certPath)
			}
			afterMeta, ok := afterNode.records[certPath]
			if !ok {
				t.Fatalf("missing post-rotation certificate metadata for pod %s path %s", afterNode.podName, certPath)
			}

			detail := fmt.Sprintf(
				"pod=%s path=%s before(serial=%s fingerprint=%s notBefore=%s notAfter=%s) after(serial=%s fingerprint=%s notBefore=%s notAfter=%s)",
				afterNode.podName,
				certPath,
				beforeMeta.serial, beforeMeta.fingerprintSHA256, beforeMeta.notBeforeRaw, beforeMeta.notAfterRaw,
				afterMeta.serial, afterMeta.fingerprintSHA256, afterMeta.notBeforeRaw, afterMeta.notAfterRaw,
			)

			if beforeMeta.fingerprintSHA256 == afterMeta.fingerprintSHA256 {
				t.Fatalf("certificate fingerprint did not change after rotation: %s", detail)
			}
			if beforeMeta.serial == afterMeta.serial {
				t.Fatalf("certificate serial did not change after rotation: %s", detail)
			}
			if afterMeta.notBefore.Before(beforeMeta.notBefore) {
				t.Fatalf("certificate notBefore moved backwards after rotation: %s", detail)
			}
		}
	}
}

// collectWorkerAgentActiveTimestamp reads each worker pod's runtime-agent
// ActiveEnterTimestampMonotonic, keyed by pod name. Workers have no server certs, so a restart is
// the only observable proof that CertificateRotation touched them.
func collectWorkerAgentActiveTimestamp(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, podNames []string, runtimeName string) map[string]uint64 {
	t.Helper()

	values := make(map[string]uint64, len(podNames))
	cmd := fmt.Sprintf("systemctl show %s-agent -p ActiveEnterTimestampMonotonic --value", runtimeName)
	for _, podName := range podNames {
		out, err := cluster.ExecOnPod(clients, fx.ns.Name, podName, "sh", "-c", cmd)
		if err != nil {
			t.Fatalf("failed reading %s-agent ActiveEnterTimestampMonotonic on pod %s: %v\noutput: %s", runtimeName, podName, err, strings.TrimSpace(out))
		}
		valueText := strings.TrimSpace(out)
		value, err := strconv.ParseUint(valueText, 10, 64)
		if err != nil {
			t.Fatalf("failed parsing %s-agent ActiveEnterTimestampMonotonic on pod %s as uint: value=%q err=%v", runtimeName, podName, valueText, err)
		}
		if value == 0 {
			t.Fatalf("invalid %s-agent ActiveEnterTimestampMonotonic on pod %s: got zero", runtimeName, podName)
		}
		values[podName] = value
	}
	return values
}

// assertWorkerAgentRestarted fails the test unless every pod's ActiveEnterTimestampMonotonic
// advanced, proving the runtime agent actually restarted.
func assertWorkerAgentRestarted(t *testing.T, before, after map[string]uint64) {
	t.Helper()

	for podName, beforeValue := range before {
		afterValue, ok := after[podName]
		if !ok {
			t.Fatalf("missing post-rotation worker agent timestamp for pod %s", podName)
		}
		if afterValue <= beforeValue {
			t.Fatalf("worker runtime agent ActiveEnterTimestampMonotonic did not advance on pod %s: before=%d after=%d", podName, beforeValue, afterValue)
		}
	}
}
