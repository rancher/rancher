package imported

import (
	"fmt"
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
	"k8s.io/apimachinery/pkg/runtime"
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

func Test_Imported_Operation_SetD_ImportedCertificateRotation(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	requiredPaths := requiredCertificatePaths(runtimeName)
	before := collectRequiredCertificateMetadata(t, fx, requiredPaths)

	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, op.Status.Step)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&op.Status))
	op = WaitForCertificateRotationSucceeded(t, cs, op, fx.mgmtCluster.Name, fx.mgmtCluster.Name)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, op.Status.Step)

	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)
	after := collectRequiredCertificateMetadata(t, fx, requiredPaths)
	assertCertificateRotationMetadata(t, before, after, requiredPaths)

	assertDownstreamAPIUsableAfterRotation(t, fx)
}

func Test_Imported_Operation_SetD_ImportedCertificateRotationLifecycleHook(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-lifecycle-hook", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	requiredPaths := requiredCertificatePaths(runtimeName)
	before := collectRequiredCertificateMetadata(t, fx, requiredPaths)

	const (
		hookName     = "v2prov-e2e-test"
		delegateName = "v2prov-e2e-test-delegate"
	)
	rotateHookKey := certificaterotation.RotateStepHookLabelPrefix + hookName
	succeededHookKey := planv1alpha1.SucceededPhaseHookLabelPrefix + hookName

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
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, final.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, final.Status.Step)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&final.Status))

	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, final)
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

	fx := setUpImportedCluster(t, cs, "test-imported-certificate-rotation-multi-node", []cluster.ImportedNodePool{
		{ETCD: true, Quantity: 1},
		{ControlPlane: true, Quantity: 1},
		{Worker: true, Quantity: 2},
	})

	runtimeName := capr.GetRuntime(defaults.SomeK8sVersion)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", runtimeName)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", runtimeName, runtimeName)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)
	expectedNodes := []string{
		"imported-init-0",
		"imported-node-1",
		"imported-node-2",
		"imported-node-3",
	}
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, kubectlEnv, expectedNodes)

	etcdCertPaths := []string{
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/etcd/server-client.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/etcd/peer-server-client.crt", runtimeName),
	}
	controlPlaneCertPaths := []string{
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/client-admin.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/serving-kube-apiserver.crt", runtimeName),
	}

	etcdPodNames := []string{fx.pods[0].Name}
	controlPlanePodNames := []string{fx.pods[1].Name}
	workerPodNames := []string{fx.pods[2].Name, fx.pods[3].Name}

	beforeEtcd := collectNodeCertificateMetadata(t, cs, fx, etcdPodNames, etcdCertPaths)
	beforeControlPlane := collectNodeCertificateMetadata(t, cs, fx, controlPlanePodNames, controlPlaneCertPaths)
	beforeWorkerAgentTimestamp := collectWorkerAgentActiveTimestamp(t, cs, fx, workerPodNames, runtimeName)

	op := RunCertificateRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, op.Status.Step)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&op.Status))

	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name
	op = WaitForCertificateRotationSucceeded(t, cs, op, beaconNS, beaconName)

	waitForImportedCertificateRotationRecovery(t, cs, fx, runtimeName, op)
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, kubectlEnv, expectedNodes)
	assertDownstreamAPIUsableAfterRotation(t, fx)

	afterEtcd := collectNodeCertificateMetadata(t, cs, fx, etcdPodNames, etcdCertPaths)
	afterControlPlane := collectNodeCertificateMetadata(t, cs, fx, controlPlanePodNames, controlPlaneCertPaths)
	afterWorkerAgentTimestamp := collectWorkerAgentActiveTimestamp(t, cs, fx, workerPodNames, runtimeName)

	assertNodeCertificateRotationMetadata(t, beforeEtcd, afterEtcd, etcdCertPaths)
	assertNodeCertificateRotationMetadata(t, beforeControlPlane, afterControlPlane, controlPlaneCertPaths)
	assertWorkerAgentRestarted(t, beforeWorkerAgentTimestamp, afterWorkerAgentTimestamp)
}

func requiredCertificatePaths(runtimeName string) []string {
	return []string{
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/client-admin.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/serving-kube-apiserver.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/etcd/server-client.crt", runtimeName),
		fmt.Sprintf("/var/lib/rancher/%s/server/tls/etcd/peer-server-client.crt", runtimeName),
	}
}

func collectRequiredCertificateMetadata(t *testing.T, fx *importedClusterFixture, paths []string) map[string]certificateMetadata {
	t.Helper()

	metadata := make(map[string]certificateMetadata, len(paths))
	for _, certPath := range paths {
		metadata[certPath] = collectCertificateMetadata(t, fx, certPath)
	}
	return metadata
}

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

func execCertificateMetadataCommandOnInitPod(t *testing.T, fx *importedClusterFixture, certPath string) (string, error) {
	t.Helper()

	cmd := buildCertificateMetadataCommand(certPath)
	return fx.execKubectl(t, cmd)
}

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

func buildCertificateMetadataCommand(certPath string) string {
	return fmt.Sprintf(
		"if [ ! -f %q ]; then echo \"missing certificate: %s\" >&2; exit 1; fi; openssl x509 -in %q -noout -serial -startdate -enddate -fingerprint -sha256",
		certPath, certPath, certPath,
	)
}

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

	notBefore, err := time.Parse("Jan 2 15:04:05 2006 MST", record.notBeforeRaw)
	if err != nil {
		return record, fmt.Errorf("parse notBefore %q: %w", record.notBeforeRaw, err)
	}
	notAfter, err := time.Parse("Jan 2 15:04:05 2006 MST", record.notAfterRaw)
	if err != nil {
		return record, fmt.Errorf("parse notAfter %q: %w", record.notAfterRaw, err)
	}
	record.notBefore = notBefore
	record.notAfter = notAfter
	return record, nil
}

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

func waitForImportedCertificateRotationRecovery(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, runtimeName string, op *opv1alpha1.CertificateRotation) {
	t.Helper()

	mgmtCluster := fx.mgmtCluster
	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleCertificateRotationError(t, clients, fx.mgmtCluster.Name, op, err)

	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", runtimeName)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", runtimeName, runtimeName)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)
	waitForImportedNodesReady(t, clients, fx.ns.Name, fx.pods[0].Name, kubectlEnv, []string{"imported-init-0"})

	if out, err := fx.execKubectl(t, "kubectl get --raw=/readyz"); err != nil {
		t.Fatalf("downstream API readyz check failed: %v\noutput: %s", err, strings.TrimSpace(out))
	}

	out, err := fx.execKubectl(t, fmt.Sprintf("%s certificate check --output table", runtimeName))
	if err != nil {
		t.Fatalf("runtime certificate check failed for %s: %v\noutput: %s", runtimeName, err, strings.TrimSpace(out))
	}
}

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
