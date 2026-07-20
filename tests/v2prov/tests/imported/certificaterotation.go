package imported

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

type conditionSummary struct {
	Type   string                 `json:"type"`
	Status corev1.ConditionStatus `json:"status"`
	Reason string                 `json:"reason,omitempty"`
}

type managementClusterSummary struct {
	Name       string             `json:"name"`
	Ready      bool               `json:"ready"`
	Conditions []conditionSummary `json:"conditions,omitempty"`
}

type beaconSummary struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Active    bool     `json:"active"`
	Owner     string   `json:"owner,omitempty"`
	Delegates []string `json:"delegates,omitempty"`
}

type certificateRotationSummary struct {
	Name             string                             `json:"name"`
	Namespace        string                             `json:"namespace"`
	Phase            opv1alpha1.OperationPhase          `json:"phase,omitempty"`
	Step             opv1alpha1.CertificateRotationStep `json:"step,omitempty"`
	LastUpdated      metav1.Time                        `json:"lastUpdated,omitempty"`
	PendingReason    string                             `json:"pendingReason,omitempty"`
	InProgressReason string                             `json:"inProgressReason,omitempty"`
	SucceededReason  string                             `json:"succeededReason,omitempty"`
	FailedReason     string                             `json:"failedReason,omitempty"`
}

type machinePlanSecretSummary struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              corev1.SecretType `json:"type,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty"`
	CreationTimestamp metav1.Time       `json:"creationTimestamp,omitempty"`
	EtcdRole          bool              `json:"etcdRole"`
	ControlPlaneRole  bool              `json:"controlPlaneRole"`
	WorkerRole        bool              `json:"workerRole"`
}

// CertificateRotationOption mutates the CertificateRotation object before it is submitted.
type CertificateRotationOption func(*opv1alpha1.CertificateRotation)

// WithCertificateRotationLabels merges the given labels onto the operation.
func WithCertificateRotationLabels(labels map[string]string) CertificateRotationOption {
	return func(op *opv1alpha1.CertificateRotation) {
		if op.Labels == nil {
			op.Labels = map[string]string{}
		}
		for k, v := range labels {
			op.Labels[k] = v
		}
	}
}

func buildCertificateRotationOp(namespace string, clusterRef corev1.ObjectReference, opts ...CertificateRotationOption) *opv1alpha1.CertificateRotation {
	op := &opv1alpha1.CertificateRotation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-certificate-rotation-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.CertificateRotationSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
				TTL:        60,
			},
		},
	}
	for _, opt := range opts {
		opt(op)
	}
	return op
}

// CreateCertificateRotationOp creates a CertificateRotation but does not wait for completion.
func CreateCertificateRotationOp(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference, opts ...CertificateRotationOption) *opv1alpha1.CertificateRotation {
	t.Helper()

	op, err := clients.Operation.CertificateRotation().Create(buildCertificateRotationOp(namespace, clusterRef, opts...))
	if err != nil {
		t.Fatal(err)
	}
	return op
}

// RunCertificateRotationOperationTest creates a CertificateRotation operation targeting the given
// clusterRef and waits for it to reach the Succeeded phase.
func RunCertificateRotationOperationTest(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference, opts ...CertificateRotationOption) *opv1alpha1.CertificateRotation {
	t.Helper()

	op, err := clients.Operation.CertificateRotation().Create(buildCertificateRotationOp(namespace, clusterRef, opts...))
	if err != nil {
		t.Fatal(err)
	}

	err = wait.ObjectWithTimeout(clients.Ctx, 25*time.Minute, clients.Operation.CertificateRotation().Watch, op, func(obj runtime.Object) (bool, error) {
		op = obj.(*opv1alpha1.CertificateRotation)
		if op.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("certificate rotation operation failed at step %q", op.Status.Step)
		}
		return op.Status.Phase == opv1alpha1.OperationPhaseSucceeded, nil
	})
	if err != nil {
		t.Logf(
			"certificate rotation wait failed for %s/%s: phase=%q step=%q lastUpdated=%q reasons=%s",
			op.Namespace,
			op.Name,
			op.Status.Phase,
			op.Status.Step,
			formatOperationLastUpdated(op.Status.LastUpdated),
			certificateRotationConditionReasons(&op.Status),
		)
		handleCertificateRotationError(t, clients, clusterRef.Name, op, err)
	}

	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	return op
}

func formatOperationLastUpdated(ts metav1.Time) string {
	if ts.IsZero() {
		return "<zero>"
	}
	return ts.UTC().Format(time.RFC3339)
}

func certificateRotationConditionReasons(status *opv1alpha1.CertificateRotationStatus) string {
	var reasons []string
	if reason := opv1alpha1.PendingCondition.GetReason(status); reason != "" {
		reasons = append(reasons, fmt.Sprintf("Pending=%s", reason))
	}
	if reason := opv1alpha1.InProgressCondition.GetReason(status); reason != "" {
		reasons = append(reasons, fmt.Sprintf("InProgress=%s", reason))
	}
	if reason := opv1alpha1.SucceededCondition.GetReason(status); reason != "" {
		reasons = append(reasons, fmt.Sprintf("Succeeded=%s", reason))
	}
	if reason := opv1alpha1.FailedCondition.GetReason(status); reason != "" {
		reasons = append(reasons, fmt.Sprintf("Failed=%s", reason))
	}
	if len(reasons) == 0 {
		return "none"
	}
	return strings.Join(reasons, ",")
}

func handleCertificateRotationError(t *testing.T, clients *clients.Clients, name string, op *opv1alpha1.CertificateRotation, err error) {
	if err == nil {
		return
	}

	objs := map[string]any{}

	c, newErr := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
	if newErr != nil {
		logrus.Error(newErr)
	} else {
		objs["mgmtCluster"] = summarizeManagementCluster(c)

		beacon, newErr := clients.Plan.Beacon().Get(c.Name, c.Name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["beacon"] = summarizeBeacon(beacon)
		}

		secrets, newErr := clients.Core.Secret().List(c.Name, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name),
			FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
		})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["machinePlans"] = summarizeMachinePlanSecrets(secrets.Items)
		}
	}

	if op != nil {
		latest, newErr := clients.Operation.CertificateRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Error(newErr)
			objs["certificateRotation"] = summarizeCertificateRotation(op)
		} else {
			objs["certificateRotation"] = summarizeCertificateRotation(latest)
		}
	}

	data, newErr := snapshotutil.CompressInterface(objs)
	if newErr != nil {
		logrus.Error(newErr)
	}
	//nolint:revive
	err = fmt.Errorf("cluster %s certificate rotation wait failed on: %w\ncluster %s test data bundle: \n%s\n", name, err, name, data)
	t.Fatal(err)
}

func summarizeMachinePlanSecrets(secrets []corev1.Secret) []machinePlanSecretSummary {
	summaries := make([]machinePlanSecretSummary, 0, len(secrets))
	for i := range secrets {
		secret := secrets[i]
		summaries = append(summaries, machinePlanSecretSummary{
			Name:              secret.Name,
			Namespace:         secret.Namespace,
			Type:              secret.Type,
			ResourceVersion:   secret.ResourceVersion,
			CreationTimestamp: secret.CreationTimestamp,
			EtcdRole:          secret.Labels[capr.EtcdRoleLabel] == "true",
			ControlPlaneRole:  secret.Labels[capr.ControlPlaneRoleLabel] == "true",
			WorkerRole:        secret.Labels[capr.WorkerRoleLabel] == "true",
		})
	}
	return summaries
}

func summarizeManagementCluster(cluster *v3.Cluster) managementClusterSummary {
	return managementClusterSummary{
		Name:       cluster.Name,
		Ready:      v3.Ready.IsTrue(cluster),
		Conditions: summarizeClusterConditions(cluster.Status.Conditions),
	}
}

func summarizeClusterConditions(conditions []v3.ClusterCondition) []conditionSummary {
	summaries := make([]conditionSummary, 0, len(conditions))
	for i := range conditions {
		summaries = append(summaries, conditionSummary{
			Type:   string(conditions[i].Type),
			Status: conditions[i].Status,
			Reason: conditions[i].Reason,
		})
	}
	return summaries
}

func summarizeBeacon(beacon *planv1alpha1.Beacon) beaconSummary {
	return beaconSummary{
		Name:      beacon.Name,
		Namespace: beacon.Namespace,
		Active:    beacon.Status.Active,
		Owner:     beacon.Status.Owner,
		Delegates: beacon.Status.Delegates,
	}
}

func summarizeCertificateRotation(op *opv1alpha1.CertificateRotation) certificateRotationSummary {
	if op == nil {
		return certificateRotationSummary{}
	}
	return certificateRotationSummary{
		Name:             op.Name,
		Namespace:        op.Namespace,
		Phase:            op.Status.Phase,
		Step:             op.Status.Step,
		LastUpdated:      op.Status.LastUpdated,
		PendingReason:    opv1alpha1.PendingCondition.GetReason(&op.Status),
		InProgressReason: opv1alpha1.InProgressCondition.GetReason(&op.Status),
		SucceededReason:  opv1alpha1.SucceededCondition.GetReason(&op.Status),
		FailedReason:     opv1alpha1.FailedCondition.GetReason(&op.Status),
	}
}

// CertificateRotationCheckpoint is the state captured when WaitForCertificateRotationHookPause
// confirms a hook has fired.
type CertificateRotationCheckpoint struct {
	Op     *opv1alpha1.CertificateRotation
	Beacon *planv1alpha1.Beacon
}

// WaitForCertificateRotationHookPause polls until the named hook on the op has fired.
func WaitForCertificateRotationHookPause(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.CertificateRotation,
	beaconNS, beaconName, hookLabelKey, delegateName string,
	expectedPhase opv1alpha1.OperationPhase,
	expectedStep opv1alpha1.CertificateRotationStep,
) CertificateRotationCheckpoint {
	t.Helper()

	var checkpoint CertificateRotationCheckpoint
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 25*time.Minute, true, func(_ context.Context) (bool, error) {
		latestOp, err := clients.Operation.CertificateRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if latestOp.Status.Phase == opv1alpha1.OperationPhaseFailed && expectedPhase != opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("operation %s/%s reached Failed phase before hook %q fired: step=%q",
				latestOp.Namespace, latestOp.Name, hookLabelKey, latestOp.Status.Step)
		}
		if latestOp.Status.Phase != expectedPhase {
			return false, nil
		}
		if expectedStep != "" && latestOp.Status.Step != expectedStep {
			return false, nil
		}
		if _, ok := latestOp.Labels[hookLabelKey]; !ok {
			return false, nil
		}

		beacon, err := clients.Plan.Beacon().Get(beaconNS, beaconName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !plan.IsInDelegateChain(beacon, delegateName) {
			return false, nil
		}

		checkpoint = CertificateRotationCheckpoint{Op: latestOp, Beacon: beacon}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for hook %q to pause op %s/%s at phase=%q step=%q: %v",
			hookLabelKey, op.Namespace, op.Name, expectedPhase, expectedStep, err)
	}
	return checkpoint
}

// AdvancePastCertificateRotationHook clears the hook label and pops the delegate.
func AdvancePastCertificateRotationHook(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.CertificateRotation,
	beaconNS, beaconName, hookLabelKey, delegateName string,
) {
	t.Helper()

	latestOp, err := clients.Operation.CertificateRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get op %s/%s: %v", op.Namespace, op.Name, err)
	}
	latestOp = latestOp.DeepCopy()
	delete(latestOp.Labels, hookLabelKey)
	if _, err := clients.Operation.CertificateRotation().Update(latestOp); err != nil {
		t.Fatalf("clear hook label %q on op %s/%s: %v", hookLabelKey, op.Namespace, op.Name, err)
	}

	beacon, err := clients.Plan.Beacon().Get(beaconNS, beaconName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get beacon %s/%s: %v", beaconNS, beaconName, err)
	}
	if _, err := plan.PopDelegate(beacon, delegateName, clients.Plan.Beacon()); err != nil {
		t.Fatalf("pop delegate %q from beacon %s/%s: %v", delegateName, beaconNS, beaconName, err)
	}
}

// WaitForCertificateRotationSucceeded polls until the op reaches Succeeded with no delegates left.
func WaitForCertificateRotationSucceeded(t *testing.T, clients *clients.Clients, op *opv1alpha1.CertificateRotation, beaconNS, beaconName string) *opv1alpha1.CertificateRotation {
	t.Helper()

	var latestOp *opv1alpha1.CertificateRotation
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 10*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err := clients.Operation.CertificateRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if got.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("operation reached Failed phase at step %q", got.Status.Step)
		}
		if got.Status.Phase != opv1alpha1.OperationPhaseSucceeded {
			return false, nil
		}

		beacon, err := clients.Plan.Beacon().Get(beaconNS, beaconName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(beacon.Status.Delegates) > 0 {
			return false, nil
		}

		latestOp = got
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for op %s/%s to reach Succeeded with empty delegate chain: %v", op.Namespace, op.Name, err)
	}
	return latestOp
}
