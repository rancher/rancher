package etcdsnapshotrestore

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const ControllerOwnerKey = "etcd-snapshot-restore"

const waitForPodListScript = `#!/bin/sh

i=0

while [ $i -lt 30 ]; do
	if $@ >/dev/null 2>&1; then
		exit 0
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`

type handler struct {
	etcdsnapshotrestores operationcontrollers.ETCDSnapshotRestoreController

	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	secrets     corecontrollers.SecretClient
	secretCache corecontrollers.SecretCache

	store *planapi.Store

	dynamic *dynamic.Controller

	clients *wrangler.CAPIContext
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		etcdsnapshotrestores: clients.Operation.ETCDSnapshotRestore(),
		beacons:              clients.Plan.Beacon(),
		beaconCache:          clients.Plan.Beacon().Cache(),
		secrets:              clients.Core.Secret(),
		secretCache:          clients.Core.Secret().Cache(),
		dynamic:              clients.Dynamic,
		store:                planapi.NewStore(clients.Core.Secret()),
		clients:              clients,
	}

	operationcontrollers.RegisterETCDSnapshotRestoreStatusHandler(ctx, clients.Operation.ETCDSnapshotRestore(), "", "etcd-snapshot-restore-handler", h.OnChange)
}

func (h *handler) OnChange(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if reflect.DeepEqual(op.Status, status) {
		h.etcdsnapshotrestores.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
	}
	return status, nil
}

func (h *handler) onChange(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	if op == nil {
		return status, nil
	}

	if op.DeletionTimestamp != nil {
		return status, nil
	}

	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: skipping paused operation", op.Namespace, op.Name)
		return status, nil
	}

	if status.Phase == "" {
		status.Phase = opv1alpha1.OperationPhasePending
		status.LastUpdated = metav1.Now()
	}

	gvk := schema.FromAPIVersionAndKind(op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
	ref, err := h.dynamic.Get(gvk, op.Spec.ClusterRef.Namespace, op.Spec.ClusterRef.Name)
	if apierrors.IsNotFound(err) {
		key := fmt.Sprintf("apiVersion=%s, kind=%s", op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
		if op.Spec.ClusterRef.Namespace != "" {
			key += fmt.Sprintf(", namespace=%s", op.Spec.ClusterRef.Namespace)
		}
		key += fmt.Sprintf(", name=%s", op.Spec.ClusterRef.Name)
		logrus.Errorf("[etcdsnapshotrestore]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.ClusterNotFoundReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("cluster %s not found", key))

		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()
		return status, nil
	}
	if err != nil {
		return status, err
	}

	ustrMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ref)
	if err != nil {
		return status, err
	}

	ustr := unstructured.Unstructured{Object: ustrMap}

	namespace := op.Spec.ClusterRef.Namespace
	if namespace == "" {
		mapping, err := h.clients.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return status, err
		}
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			namespace = op.Namespace
		} else {
			namespace = op.Spec.ClusterRef.Name
		}
	}

	beacon, err := h.beacons.Get(namespace, ustr.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		key := fmt.Sprintf("apiVersion=%s, kind=%s", ustr.GetAPIVersion(), ustr.GetKind())
		if ustr.GetNamespace() != "" {
			key += fmt.Sprintf(", namespace=%s", ustr.GetNamespace())
		}
		key += fmt.Sprintf(", name=%s", ustr.GetName())
		logrus.Warnf("[etcdsnapshotrestore]: %s/%s failed to find beacon for %s", op.Namespace, op.Name, key)

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")

		return status, nil
	} else if err != nil {
		return status, err
	}

	a, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return status, err
	}

	s := &scope{
		op:         op,
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: &ustr,
		adapter:    a,
	}

	if status.Phase == opv1alpha1.OperationPhasePending {
		return h.handlePending(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseInProgress {
		return h.handleInProgress(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseCanceled {
		return status, nil
	}
	if status.Phase == opv1alpha1.OperationPhaseFailed {
		return h.handleFailed(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		return h.handleSucceeded(s, status)
	}

	// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
	if ops.IsTerminal(status.Phase) && ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) {
		err = h.etcdsnapshotrestores.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
		if err != nil {
			return status, err
		}
		return status, generic.ErrSkip
	}

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))

	return status, nil
}

type scope struct {
	op        *opv1alpha1.ETCDSnapshotRestore
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

func (h *handler) handlePending(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	beacon, err := planapi.AcquireBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	if beacon == nil {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")
		return status, nil
	}
	logrus.Infof("[etcdsnapshotrestore] %s/%s: acquired beacon, waiting for agents to register", s.op.Namespace, s.op.Name)

	if ok, err := s.adapter.WaitForRegister(); err != nil {
		return status, err
	} else if !ok {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for system-agents to connect", s.op.Namespace, s.op.Name)
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForRegistrationReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for system-agents to connect")
		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to shutdown", s.op.Namespace, s.op.Name)

	status.Phase = opv1alpha1.OperationPhaseInProgress
	status.LastUpdated = metav1.Now()
	status.Step = opv1alpha1.ETCDSnapshotRestoreStepShutdown

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	if !planapi.HoldingBeacon(s.beacon, ControllerOwnerKey) {
		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "Beacon acquired by another controller, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = planapi.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	switch s.op.Status.Step {
	case opv1alpha1.ETCDSnapshotRestoreStepShutdown:
		return h.reconcileShutdown(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepRestore:
		return h.reconcileRestore(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup:
		return h.reconcilePostRestorePodCleanup(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster:
		return h.reconcileRestartCluster(s, status, opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup)
	case opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup:
		return h.reconcilePostRestoreNodeCleanup(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepRestartCluster:
		return h.reconcileRestartCluster(s, status, "")
	}

	status.Phase = opv1alpha1.OperationPhaseFailed
	status.LastUpdated = metav1.Now()

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownStepReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("current step [\"%s\"] is unknown, expected one of: [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]",
		status.Step,
		opv1alpha1.ETCDSnapshotRestoreStepShutdown,
		opv1alpha1.ETCDSnapshotRestoreStepRestore,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepRestartCluster))

	return status, nil
}

func (h *handler) reconcileShutdown(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling shutdown", s.op.Namespace, s.op.Name)

	secrets, err := planapi.NewLabeler().
		And(planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.CattleOSLabel, capr.DefaultMachineOS)).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	for _, secret := range secrets {
		instructions := []planapi.OneTimeInstruction{
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "shutdown",
					Command: "/bin/sh",
					Env: []string{
						fmt.Sprintf("%s_DATA_DIR=%s", strings.ToUpper(s.adapter.RuntimeCommand()), s.adapter.DataDirectory(secret)),
					},
					Args: []string{
						"-c",
						fmt.Sprintf("if [ -z $(command -v %[1]s) ] && [ -z $(command -v %[2]s) ]; then echo %[1]s does not appear to be installed; exit 0; else %[2]s; fi",
							s.adapter.RuntimeCommand(),
							s.adapter.RuntimeCommand()+"-killall.sh"),
					},
				},
			},
		}

		if secret.Labels[capr.EtcdRoleLabel] == "true" {
			instructions = append(instructions, planapi.OneTimeInstruction{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "create-etcd-tombstone",
					Command: "touch",
					Args:    []string{path.Join(s.adapter.DataDirectory(secret), "server/db/etcd/tombstone")},
				},
			})
		}

		if secret.Labels[capr.EtcdRoleLabel] == "true" || secret.Labels[capr.ControlPlaneRoleLabel] == "true" {
			instructions = append(instructions,
				planapi.OneTimeInstruction{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "remove-tls-directory",
						Command: "rm",
						Args:    []string{"-rf", path.Join(s.adapter.DataDirectory(secret), "server/tls")},
					},
				},
				planapi.OneTimeInstruction{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "remove-cred-directory",
						Command: "rm",
						Args:    []string{"-rf", path.Join(s.adapter.DataDirectory(secret), "server/cred")},
					},
				},
			)
		}

		nodePlan := &planapi.Plan{
			OneTimeInstructions: instructions,
		}

		planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: shutdown failed for %s/%s",
				s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.Phase = opv1alpha1.OperationPhaseFailed
			status.LastUpdated = metav1.Now()

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("shutdown failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for shutdown: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to restore", s.op.Namespace, s.op.Name)

	status.Step = opv1alpha1.ETCDSnapshotRestoreStepRestore
	return status, nil
}

func (h *handler) reconcileRestore(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling etcd restore", s.op.Namespace, s.op.Name)

	snapshotName := s.op.Spec.Args.Name
	if snapshotName == "" {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: snapshot name is required for etcd restore", s.op.Namespace, s.op.Name)

		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "snapshot name is required for etcd restore")

		return status, nil
	}

	secrets, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.EtcdRoleLabel, "true"),
			planapi.Label(capr.InitNodeLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	if len(secrets) == 0 {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: no etcd nodes found for restore", s.op.Namespace, s.op.Name)

		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "no etcd nodes found for restore")

		return status, nil
	}

	secret := secrets[0]

	nodePlan := &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "remove-etcd-db-dir",
					Command: "rm",
					Args:    []string{"-rf", path.Join(s.adapter.DataDirectory(secret), "server/db/etcd")},
				},
			},
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "restore",
					Command: s.adapter.RuntimeCommand(),
					Args: []string{
						"server",
						"--cluster-reset",
						"--etcd-arg=advertise-client-urls=https://127.0.0.1:2379",
						"--etcd-disable-snapshots=false",
						fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshotName),
						"--etcd-s3=false",
					},
				},
			},
		},
	}

	planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: etcd restore failed for %s/%s",
			s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("etcd restore failed for %s/%s", secret.Namespace, secret.Name))

		return status, nil
	}

	if wait, msg := planStatus.Wait(); wait {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for etcd restore: %s", s.op.Namespace, s.op.Name, msg)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, msg)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to post-restore pod cleanup", s.op.Namespace, s.op.Name)

	status.Step = opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup
	return status, nil
}

func (h *handler) reconcilePostRestorePodCleanup(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling post-restore pod cleanup", s.op.Namespace, s.op.Name)

	secrets, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.ControlPlaneRoleLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	if len(secrets) == 0 {
		secrets, err = planapi.NewLabeler().
			And(
				planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
				planapi.Label(capr.EtcdRoleLabel, "true")).
			WithSorter(planapi.DefaultSorter()).
			Collect(h.secretCache, s.namespace)
		if err != nil {
			return status, err
		}
	}

	if len(secrets) == 0 {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: no suitable nodes found for pod cleanup, skipping", s.op.Namespace, s.op.Name)
		status.Step = opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster
		return status, nil
	}

	secret := secrets[0]

	kubectl := s.adapter.KubectlPath(secret)
	kubeconfig := s.adapter.KubeconfigPath(secret)

	podSelectors := []string{
		"kube-system:k8s-app=kube-dns",
		"kube-system:k8s-app=kube-dns-autoscaler",
	}

	if s.adapter.RuntimeCommand() == "rke2" {
		podSelectors = append(podSelectors,
			"kube-system:app=rke2-metrics-server",
			"tigera-operator:k8s-app=tigera-operator",
			"calico-system:k8s-app=calico-node",
			"calico-system:k8s-app=calico-kube-controllers",
			"calico-system:k8s-app=calico-typha",
			"kube-system:k8s-app=canal",
			"kube-system:k8s-app=cilium",
			"kube-system:app=rke2-multus",
			"kube-system:app.kubernetes.io/name=rke2-ingress-nginx",
		)
	}

	waitScriptPath := path.Join(s.adapter.DataDirectory(secret), "capr/etcd-restore/bin/wait_for_pod_list.sh")

	instructions := []planapi.OneTimeInstruction{
		{
			CommonInstruction: planapi.CommonInstruction{
				Name:    "wait-for-api-server",
				Command: "/bin/sh",
				Args: []string{
					"-x",
					waitScriptPath,
					kubectl,
					"--kubeconfig",
					kubeconfig,
					"get",
					"pods",
					"--all-namespaces",
				},
			},
		},
	}

	for i, podSelector := range podSelectors {
		if namespace, labelSelector, ok := strings.Cut(podSelector, ":"); ok {
			instructions = append(instructions, planapi.OneTimeInstruction{
				CommonInstruction: planapi.CommonInstruction{
					Name:    fmt.Sprintf("cleanup-pods-%d", i),
					Command: kubectl,
					Args: []string{
						"--kubeconfig",
						kubeconfig,
						"delete",
						"pods",
						"-n",
						namespace,
						"-l",
						labelSelector,
						"--wait=false",
					},
				},
			})
		}
	}

	nodePlan := &planapi.Plan{
		Files: []planapi.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(waitForPodListScript)),
				Path:    waitScriptPath,
				Dynamic: true,
			},
		},
		OneTimeInstructions: instructions,
	}

	planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: pod cleanup failed for %s/%s",
			s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("post-restore pod cleanup failed for %s/%s", secret.Namespace, secret.Name))

		return status, nil
	}

	if wait, msg := planStatus.Wait(); wait {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for pod cleanup: %s", s.op.Namespace, s.op.Name, msg)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, msg)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to initial restart", s.op.Namespace, s.op.Name)

	status.Step = opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster
	return status, nil
}

func (h *handler) reconcileRestartCluster(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus, nextStep opv1alpha1.ETCDSnapshotRestoreStep) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling cluster restart", s.op.Namespace, s.op.Name)

	secrets, err := planapi.NewLabeler().
		And(planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.CattleOSLabel, capr.DefaultMachineOS)).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret)
		if err != nil {
			return status, err
		}

		unit := s.adapter.ServerUnit()
		if secret.Labels[capr.EtcdRoleLabel] != "true" && secret.Labels[capr.ControlPlaneRoleLabel] != "true" {
			unit = s.adapter.RuntimeCommand() + "-agent"
		}

		nodePlan := &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "restart",
						Command: "systemctl",
						Args:    []string{"restart", unit},
					},
				},
			},
			Probes: probes,
		}

		planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: restart failed for %s/%s",
				s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.Phase = opv1alpha1.OperationPhaseFailed
			status.LastUpdated = metav1.Now()

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("restart failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for restart: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	if nextStep != "" {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to %s", s.op.Namespace, s.op.Name, nextStep)
		status.Step = nextStep
		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.Phase = opv1alpha1.OperationPhaseSucceeded
	status.LastUpdated = metav1.Now()

	opv1alpha1.SucceededCondition.True(&status)
	opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(&status, "Operation completed successfully")

	return status, nil
}

func (h *handler) reconcilePostRestoreNodeCleanup(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: node cleanup deferred to cluster reconciliation", s.op.Namespace, s.op.Name)

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to final restart", s.op.Namespace, s.op.Name)

	status.Step = opv1alpha1.ETCDSnapshotRestoreStepRestartCluster
	return status, nil
}

func (h *handler) handleFailed(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	err := planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	return status, nil
}

func (h *handler) handleSucceeded(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	if planapi.HoldingBeacon(s.beacon, ControllerOwnerKey) {
		var err error
		s.beacon, err = planapi.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		err = planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
		if err != nil {
			return status, err
		}

		// enqueue original object to ensure it is processed by requisite controllers
		gvk := schema.FromAPIVersionAndKind(s.clusterObj.GetAPIVersion(), s.clusterObj.GetKind())
		_ = h.dynamic.Enqueue(gvk, s.clusterObj.GetNamespace(), s.clusterObj.GetName())
	}
	return status, nil
}

// updateStatus updates the conditions of the operation based on the current status.
// This function also updates the ObservedGeneration.
// The handler is responsible for updating the condition relevant to the current phase, but this function updates the
// remaining conditions.
func updateStatus(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) opv1alpha1.ETCDSnapshotRestoreStatus {
	logrus.Tracef("[etcdsnapshotrestore] %s/%s: updating conditions", op.Namespace, op.Name)

	status.ObservedGeneration = op.Generation

	if status.Phase == opv1alpha1.OperationPhasePending {
		opv1alpha1.PendingCondition.True(&status)
	} else if status.Phase == opv1alpha1.OperationPhaseInProgress {
		opv1alpha1.PendingCondition.False(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.InProgressReason)
		opv1alpha1.PendingCondition.Message(&status, "Operation now in progress")
	} else if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		opv1alpha1.PendingCondition.False(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.PendingCondition.Message(&status, "Operation completed successfully")
		opv1alpha1.InProgressCondition.False(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.InProgressCondition.Message(&status, "Operation completed successfully")
		opv1alpha1.FailedCondition.False(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.NotFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "Operation completed successfully")
	} else if status.Phase == opv1alpha1.OperationPhaseFailed {
		opv1alpha1.PendingCondition.False(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.PendingCondition.Message(&status, "Operation completed successfully")
		opv1alpha1.InProgressCondition.False(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.InProgressCondition.Message(&status, "Operation failed")
		opv1alpha1.SucceededCondition.False(&status)
		opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.NotSuccessfulReason)
		opv1alpha1.SucceededCondition.Message(&status, "Operation failed")
	}

	return status
}
