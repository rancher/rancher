package systemagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/image"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

const (
	UpgradeDigestAnnotation                  = "upgrade.cattle.io/digest"
	AppliedSystemAgentUpgraderHashAnnotation = "management.cattle.io/applied-system-agent-upgrader-hash"
	day2OpsEnabledAnnotation                 = "operations.cattle.io/ops-enabled"
	importedDay2OpsResetInProgressAnnotation = "operations.cattle.io/imported-reset-in-progress"
	operationByClusterRefIndex               = "operations.byClusterRef"

	SystemAgentUpgraderPlanName               = "system-agent-upgrader"
	SystemAgentUpgraderWindowsPlanName        = "system-agent-upgrader-windows"
	SystemAgentUpgraderServiceAccountName     = "system-agent-upgrader"
	SystemAgentUpgraderClusterRoleName        = "system-agent-upgrader"
	SystemAgentUpgraderClusterRoleBindingName = "system-agent-upgrader"
	resetBeaconOwnerKey                       = "imported-day2ops-reset"

	importedResetStartedReason             = "ResetStarted"
	importedResetWaitingForBeaconReason    = "WaitingForBeaconRelease"
	importedResetCleaningOperationsReason  = "CleaningOperations"
	importedResetCleaningBeaconReason      = "CleaningBeacon"
	importedResetCleaningMachinePlans      = "CleaningMachinePlans"
	importedResetWaitingForClusterAPI      = "WaitingForClusterAPI"
	importedResetWaitingForUninstallReason = "WaitingForUninstall"
	importedResetCleaningSUCReason         = "CleaningSUCResources"
	importedResetCompleteReason            = "ResetComplete"
)

var (
	// installCounter keeps track of the number of clusters for which the handler is concurrently installing or upgrading
	// the resources needed for upgrading system-agent.
	installCounter atomic.Int32

	importedDay2OpsResetCondition = condition.Cond("ImportedDay2OpsReset")
	upgradePlanGVR                = schema.GroupVersionResource{Group: "upgrade.cattle.io", Version: "v1", Resource: "plans"}
)

type handler struct {
	ctx     context.Context
	manager *clustermanager.Manager

	clusters    mgmtcontrollers.ClusterController
	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	etcdSnapshotSaves        operationcontrollers.ETCDSnapshotSaveClient
	etcdSnapshotSaveCache    operationcontrollers.ETCDSnapshotSaveCache
	etcdSnapshotRestores     operationcontrollers.ETCDSnapshotRestoreClient
	etcdSnapshotRestoreCache operationcontrollers.ETCDSnapshotRestoreCache
	encryptionRotations      operationcontrollers.EncryptionKeyRotationClient
	encryptionRotationCache  operationcontrollers.EncryptionKeyRotationCache
	secrets                  corecontrollers.SecretClient
	secretCache              corecontrollers.SecretCache
}

func clusterRefIndexKey(ref *corev1.ObjectReference) string {
	if ref == nil || ref.APIVersion == "" || ref.Kind == "" || ref.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, ref.Namespace, ref.Name)
}

func importedClusterRefIndexKey(clusterName string) string {
	if clusterName == "" {
		return ""
	}
	return clusterRefIndexKey(&corev1.ObjectReference{
		APIVersion: apimgmtv3.SchemeGroupVersion.String(),
		Kind:       "Cluster",
		Name:       clusterName,
	})
}

func Register(ctx context.Context, w *wrangler.Context, manager *clustermanager.Manager) {
	h := &handler{
		ctx:                      ctx,
		manager:                  manager,
		clusters:                 w.Mgmt.Cluster(),
		beacons:                  w.Plan.Beacon(),
		beaconCache:              w.Plan.Beacon().Cache(),
		etcdSnapshotSaves:        w.Operation.ETCDSnapshotSave(),
		etcdSnapshotSaveCache:    w.Operation.ETCDSnapshotSave().Cache(),
		etcdSnapshotRestores:     w.Operation.ETCDSnapshotRestore(),
		etcdSnapshotRestoreCache: w.Operation.ETCDSnapshotRestore().Cache(),
		encryptionRotations:      w.Operation.EncryptionKeyRotation(),
		encryptionRotationCache:  w.Operation.EncryptionKeyRotation().Cache(),
		secrets:                  w.Core.Secret(),
		secretCache:              w.Core.Secret().Cache(),
	}
	w.Operation.ETCDSnapshotSave().Cache().AddIndexer(operationByClusterRefIndex, func(obj *opv1alpha1.ETCDSnapshotSave) ([]string, error) {
		if key := clusterRefIndexKey(obj.Spec.ClusterRef); key != "" {
			return []string{key}, nil
		}
		return nil, nil
	})
	w.Operation.ETCDSnapshotRestore().Cache().AddIndexer(operationByClusterRefIndex, func(obj *opv1alpha1.ETCDSnapshotRestore) ([]string, error) {
		if key := clusterRefIndexKey(obj.Spec.ClusterRef); key != "" {
			return []string{key}, nil
		}
		return nil, nil
	})
	w.Operation.EncryptionKeyRotation().Cache().AddIndexer(operationByClusterRefIndex, func(obj *opv1alpha1.EncryptionKeyRotation) ([]string, error) {
		if key := clusterRefIndexKey(obj.Spec.ClusterRef); key != "" {
			return []string{key}, nil
		}
		return nil, nil
	})
	if features.ImportedDay2Ops.Enabled() {
		w.Mgmt.Cluster().OnChange(ctx, "imported-system-agent-setup", h.onChange)
	}
}

func (h *handler) onChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if cluster.Name == "local" {
		return cluster, nil
	}

	// only applies to imported RKE2/K3s cluster
	if cluster.Status.Driver != apimgmtv3.ClusterDriverK3s && cluster.Status.Driver != apimgmtv3.ClusterDriverRke2 {
		return cluster, nil
	}

	if cluster.Annotations[day2OpsEnabledAnnotation] == "" {
		if settings.ImportedClusterDay2OpsEnabledDefault.Get() == "true" {
			cluster := cluster.DeepCopy()
			if cluster.Annotations == nil {
				cluster.Annotations = map[string]string{}
			}
			cluster.Annotations[day2OpsEnabledAnnotation] = "true"
			logrus.Infof("[importedsystemagent] cluster %s: setting %s to true", cluster.Name, day2OpsEnabledAnnotation)
			return h.clusters.Update(cluster)
		}
		return cluster, nil
	}

	// Once imported reset has started, keep reconciling reset until it reaches a safe terminal
	// point, even if ops-enabled is flipped back to true.
	if shouldReconcileImportedDisable(cluster.Annotations) {
		return h.reconcileImportedDisable(cluster)
	}
	if cluster.Annotations[day2OpsEnabledAnnotation] != "true" {
		return cluster, nil
	}
	return h.reconcileImportedEnable(cluster)
}

// reconcileImportedEnable clears any reset bookkeeping before continuing the normal install path.
func (h *handler) reconcileImportedEnable(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	var err error
	cluster, changed, err := h.clearResetMarker(cluster)
	if err != nil {
		return cluster, err
	}
	if changed {
		return cluster, nil
	}

	cluster, err = h.setResetCondition(cluster, "False", "", "")
	if err != nil {
		return cluster, err
	}

	return h.reconcileImportedInstall(cluster)
}

// reconcileImportedInstall ensures the imported system-agent upgrade resources exist and match the current template hash.
func (h *handler) reconcileImportedInstall(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	_, err := h.beaconCache.Get(cluster.Name, cluster.Name)
	if apierrors.IsNotFound(err) {
		_, err = h.beacons.Create(&planv1alpha1.Beacon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.Name,
				Namespace: cluster.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: cluster.APIVersion,
						Kind:       cluster.Kind,
						Name:       cluster.Name,
						UID:        cluster.UID,
					},
				},
			}})
		if err != nil {
			return cluster, err
		}
	} else if err != nil {
		return cluster, err
	}

	clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
	if err != nil {
		return cluster, err
	}

	if err := healthsyncer.IsAPIUp(h.ctx, clusterCtx.K8sClient.CoreV1().Namespaces()); err != nil {
		// skip further work if the cluster's API is not reachable,
		// this usually happen during cattle-cluster-agent being redeployed
		logrus.Debugf("[importedsystemagent] [%s] cluster API is not reachable, will try again", cluster.Name)
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}

	result := installer(cluster, "stv-aggregation")

	// Calculate a hash value of the templates
	data, err := json.Marshal(result)
	if err != nil {
		return cluster, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	val, ok := cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation]
	if ok && hash == val {
		logrus.Debugf("[importedsystemagent] cluster %s/%s: applied templates for system-agent-upgrader is up to date. "+
			"To trigger a force redeployment, remove the %s annotation from the corresponding management cluster object",
			cluster.Namespace, cluster.Name, AppliedSystemAgentUpgraderHashAnnotation)
		return cluster, nil
	}

	// Limit the number of cluster to be processed simultaneously
	if installCounter.Load() >= int32(settings.SystemAgentUpgraderInstallConcurrency.GetInt()) {
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}
	installCounter.Add(1)
	defer installCounter.Add(-1)

	// ensure SUC plan is installed
	apply, err := apply.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return cluster, err
	}
	err = apply.
		WithSetID("managed-system-agent").
		WithDynamicLookup().
		WithDefaultNamespace(namespaces.System).
		ApplyObjects(result...)
	if err != nil {
		return cluster, err
	}

	// Update the annotation with the latest hash value
	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation] = hash
	if _, err := h.clusters.Update(cluster); err != nil {
		return cluster, fmt.Errorf("failed to update annotation: %w", err)
	}

	return cluster, nil
}

// reconcileImportedDisable tears down imported day2ops in reset-safe order while holding the beacon for reset.
func (h *handler) reconcileImportedDisable(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	needed, err := h.resetNeeded(cluster)
	if err != nil {
		return cluster, err
	}
	if !needed {
		cluster, err = h.clearClusterAnnotations(cluster)
		if err != nil {
			return cluster, err
		}
		return h.setResetCondition(cluster, "False", importedResetCompleteReason, "")
	}

	cluster, changed, err := h.ensureResetMarker(cluster)
	if err != nil {
		return cluster, err
	}
	if changed {
		return h.waitForReset(cluster, importedResetStartedReason, "imported day2ops disable/reset has started")
	}

	beacon, err := h.beaconCache.Get(cluster.Name, cluster.Name)
	if apierrors.IsNotFound(err) {
		beacon = nil
	} else if err != nil {
		return cluster, err
	}

	// Reset waits for the current holder to finish instead of preempting it.
	if wait, message := resetBeaconWait(beacon); wait {
		return h.waitForReset(cluster, importedResetWaitingForBeaconReason, message)
	}
	// Reset takes the beacon before deleting operation CRs to block new starts while cleanup runs.
	if beacon != nil {
		acquired, err := planapi.AcquireBeacon(beacon, h.beacons, resetBeaconOwnerKey)
		if err != nil {
			return cluster, err
		}
		if acquired == nil {
			return h.waitForReset(cluster, importedResetWaitingForBeaconReason, "waiting for beacon release")
		}
	}

	if remaining, err := h.deleteOperations(cluster.Name); err != nil {
		return cluster, err
	} else if remaining {
		return h.waitForReset(cluster, importedResetCleaningOperationsReason, "waiting for imported operations to be deleted")
	}

	if remaining, err := h.deleteBeacon(cluster.Name); err != nil {
		return cluster, err
	} else if remaining {
		return h.waitForReset(cluster, importedResetCleaningBeaconReason, "waiting for imported beacon deletion")
	}

	if remaining, err := h.deleteMachinePlans(cluster.Name); err != nil {
		return cluster, err
	} else if remaining {
		return h.waitForReset(cluster, importedResetCleaningMachinePlans, "waiting for imported machine-plan secret cleanup")
	}

	clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
	if err != nil {
		return cluster, err
	}
	if err := healthsyncer.IsAPIUp(h.ctx, clusterCtx.K8sClient.CoreV1().Namespaces()); err != nil {
		return h.waitForReset(cluster, importedResetWaitingForClusterAPI, "waiting for downstream cluster API")
	}

	if err := h.applyUninstallPlans(cluster, clusterCtx); err != nil {
		return cluster, err
	}

	complete, message, err := h.uninstallComplete(clusterCtx)
	if err != nil {
		return cluster, err
	}
	if !complete {
		return h.waitForReset(cluster, importedResetWaitingForUninstallReason, message)
	}

	if remaining, err := h.deleteSUCResources(clusterCtx); err != nil {
		return cluster, err
	} else if remaining {
		return h.waitForReset(cluster, importedResetCleaningSUCReason, "waiting for imported system-agent SUC resource cleanup")
	}

	cluster, err = h.clearClusterAnnotations(cluster)
	if err != nil {
		return cluster, err
	}

	return h.setResetCondition(cluster, "False", importedResetCompleteReason, "")
}

// waitForReset records the current reset wait reason and re-enqueues the cluster for the next poll.
func (h *handler) waitForReset(cluster *apimgmtv3.Cluster, reason, message string) (*apimgmtv3.Cluster, error) {
	updated, err := h.setResetCondition(cluster, "True", reason, message)
	if err != nil {
		return cluster, err
	}
	h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
	return updated, nil
}

// resetNeeded returns true while imported day2ops resources still exist or reset bookkeeping is still present.
func (h *handler) resetNeeded(cluster *apimgmtv3.Cluster) (bool, error) {
	if cluster.Annotations[importedDay2OpsResetInProgressAnnotation] == "true" || cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation] != "" {
		return true, nil
	}

	_, err := h.beaconCache.Get(cluster.Name, cluster.Name)
	if err == nil {
		return true, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, err
	}

	hasOperations, err := h.hasOperations(cluster.Name)
	if err != nil || hasOperations {
		return hasOperations, err
	}

	machinePlans, err := h.machinePlanSecrets(cluster.Name)
	if err != nil {
		return false, err
	}
	return len(machinePlans) > 0, nil
}

// hasOperations reports whether any imported day2ops operation CRs still reference the cluster.
func (h *handler) hasOperations(clusterName string) (bool, error) {
	indexKey := importedClusterRefIndexKey(clusterName)

	saves, err := h.etcdSnapshotSaveCache.GetByIndex(operationByClusterRefIndex, indexKey)
	if err != nil {
		return false, err
	}
	if len(saves) > 0 {
		return true, nil
	}

	restores, err := h.etcdSnapshotRestoreCache.GetByIndex(operationByClusterRefIndex, indexKey)
	if err != nil {
		return false, err
	}
	if len(restores) > 0 {
		return true, nil
	}

	rotations, err := h.encryptionRotationCache.GetByIndex(operationByClusterRefIndex, indexKey)
	if err != nil {
		return false, err
	}
	return len(rotations) > 0, nil
}

// deleteOperations deletes imported operation CRs one-by-one and returns true while any are still present.
func (h *handler) deleteOperations(clusterName string) (bool, error) {
	indexKey := importedClusterRefIndexKey(clusterName)
	remaining := false

	saves, err := h.etcdSnapshotSaveCache.GetByIndex(operationByClusterRefIndex, indexKey)
	if err != nil {
		return false, err
	}
	for i := range saves {
		remaining = true
		if saves[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.etcdSnapshotSaves.Delete(saves[i].Namespace, saves[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	restores, err := h.etcdSnapshotRestoreCache.GetByIndex(operationByClusterRefIndex, indexKey)
	if err != nil {
		return false, err
	}
	for i := range restores {
		remaining = true
		if restores[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.etcdSnapshotRestores.Delete(restores[i].Namespace, restores[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	rotations, err := h.encryptionRotationCache.GetByIndex(operationByClusterRefIndex, indexKey)
	if err != nil {
		return false, err
	}
	for i := range rotations {
		remaining = true
		if rotations[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.encryptionRotations.Delete(rotations[i].Namespace, rotations[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	return remaining, nil
}

// deleteMachinePlans deletes imported machine-plan secrets and returns true while any are still present.
func (h *handler) deleteMachinePlans(clusterName string) (bool, error) {
	secrets, err := h.machinePlanSecrets(clusterName)
	if err != nil {
		return false, err
	}
	if len(secrets) == 0 {
		return false, nil
	}
	for i := range secrets {
		if secrets[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.secrets.Delete(secrets[i].Namespace, secrets[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return true, nil
}

// machinePlanSecrets lists the imported machine-plan secrets for the cluster namespace.
func (h *handler) machinePlanSecrets(clusterName string) ([]*corev1.Secret, error) {
	secrets, err := h.secretCache.List(clusterName, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: clusterName,
	}))
	if err != nil {
		return nil, err
	}
	machinePlans := make([]*corev1.Secret, 0, len(secrets))
	for i := range secrets {
		if secrets[i].Type == capr.SecretTypeMachinePlan {
			machinePlans = append(machinePlans, secrets[i])
		}
	}
	return machinePlans, nil
}

// deleteBeacon deletes the imported beacon and returns true while it is still present.
func (h *handler) deleteBeacon(clusterName string) (bool, error) {
	beacon, err := h.beacons.Get(clusterName, clusterName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if beacon.DeletionTimestamp == nil {
		if err := h.beacons.Delete(clusterName, clusterName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return true, nil
}

// applyUninstallPlans renders and applies the imported system-agent uninstall plans downstream.
func (h *handler) applyUninstallPlans(cluster *apimgmtv3.Cluster, clusterCtx *config.UserContext) error {
	result := uninstaller(cluster, "stv-aggregation")

	applier, err := apply.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return err
	}
	return applier.
		WithSetID("managed-system-agent").
		WithDynamicLookup().
		WithDefaultNamespace(namespaces.System).
		ApplyObjects(result...)
}

// uninstallComplete waits for both uninstall plans to exist, complete, and finish applying on every node.
func (h *handler) uninstallComplete(clusterCtx *config.UserContext) (bool, string, error) {
	for _, name := range []string{SystemAgentUpgraderPlanName, SystemAgentUpgraderWindowsPlanName} {
		plan, err := h.getDownstreamPlan(clusterCtx, name)
		if err != nil {
			return false, "", err
		}
		if plan == nil {
			return false, fmt.Sprintf("waiting for uninstall plan %s to be created", name), nil
		}
		if !upgradev1.PlanComplete.IsTrue(plan) {
			msg := upgradev1.PlanComplete.GetMessage(plan)
			if msg == "" {
				msg = upgradev1.PlanComplete.GetReason(plan)
			}
			if msg == "" {
				msg = "plan not complete yet"
			}
			return false, fmt.Sprintf("waiting for uninstall plan %s completion: %s", name, msg), nil
		}
		if len(plan.Status.Applying) > 0 {
			return false, fmt.Sprintf("waiting for uninstall plan %s to finish applying on nodes: %s", name, strings.Join(plan.Status.Applying, ",")), nil
		}
	}
	return true, "", nil
}

// getDownstreamPlan reads a downstream SUC plan through the dynamic client so reset can poll its status.
func (h *handler) getDownstreamPlan(clusterCtx *config.UserContext, name string) (*upgradev1.Plan, error) {
	dynamicClient, err := dynamic.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return nil, err
	}

	obj, err := dynamicClient.Resource(upgradePlanGVR).Namespace(namespaces.System).Get(h.ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var plan upgradev1.Plan
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// deleteSUCResources deletes the downstream SUC plans and RBAC resources and returns true while any remain.
func (h *handler) deleteSUCResources(clusterCtx *config.UserContext) (bool, error) {
	dynamicClient, err := dynamic.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return false, err
	}

	remaining := false
	for _, name := range []string{SystemAgentUpgraderPlanName, SystemAgentUpgraderWindowsPlanName} {
		obj, err := dynamicClient.Resource(upgradePlanGVR).Namespace(namespaces.System).Get(h.ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return false, err
		}
		remaining = true
		if obj.GetDeletionTimestamp() == nil {
			if err := dynamicClient.Resource(upgradePlanGVR).Namespace(namespaces.System).Delete(h.ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
		}
	}

	serviceAccount, err := clusterCtx.K8sClient.CoreV1().ServiceAccounts(namespaces.System).Get(h.ctx, SystemAgentUpgraderServiceAccountName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if err == nil {
		remaining = true
		if serviceAccount.DeletionTimestamp == nil {
			if err := clusterCtx.K8sClient.CoreV1().ServiceAccounts(namespaces.System).Delete(h.ctx, SystemAgentUpgraderServiceAccountName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
		}
	}

	clusterRole, err := clusterCtx.K8sClient.RbacV1().ClusterRoles().Get(h.ctx, SystemAgentUpgraderClusterRoleName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if err == nil {
		remaining = true
		if clusterRole.DeletionTimestamp == nil {
			if err := clusterCtx.K8sClient.RbacV1().ClusterRoles().Delete(h.ctx, SystemAgentUpgraderClusterRoleName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
		}
	}

	clusterRoleBinding, err := clusterCtx.K8sClient.RbacV1().ClusterRoleBindings().Get(h.ctx, SystemAgentUpgraderClusterRoleBindingName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if err == nil {
		remaining = true
		if clusterRoleBinding.DeletionTimestamp == nil {
			if err := clusterCtx.K8sClient.RbacV1().ClusterRoleBindings().Delete(h.ctx, SystemAgentUpgraderClusterRoleBindingName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
		}
	}

	return remaining, nil
}

// ensureResetMarker marks reset as in progress so disable reconciliation stays sticky across subsequent updates.
func (h *handler) ensureResetMarker(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, bool, error) {
	if cluster.Annotations[importedDay2OpsResetInProgressAnnotation] == "true" {
		return cluster, false, nil
	}
	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[importedDay2OpsResetInProgressAnnotation] = "true"
	updated, err := h.clusters.Update(cluster)
	if err != nil {
		return cluster, false, err
	}
	return updated, true, nil
}

// clearResetMarker removes the in-progress reset marker once the controller returns to the enable path.
func (h *handler) clearResetMarker(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, bool, error) {
	if cluster.Annotations[importedDay2OpsResetInProgressAnnotation] != "true" {
		return cluster, false, nil
	}
	cluster = cluster.DeepCopy()
	delete(cluster.Annotations, importedDay2OpsResetInProgressAnnotation)
	updated, err := h.clusters.Update(cluster)
	if err != nil {
		return cluster, false, err
	}
	return updated, true, nil
}

// clearClusterAnnotations removes imported reset bookkeeping while preserving the user's ops-enabled choice.
func (h *handler) clearClusterAnnotations(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster.Annotations == nil {
		return cluster, nil
	}
	if cluster.Annotations[importedDay2OpsResetInProgressAnnotation] == "" &&
		cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation] == "" {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	delete(cluster.Annotations, importedDay2OpsResetInProgressAnnotation)
	delete(cluster.Annotations, AppliedSystemAgentUpgraderHashAnnotation)
	return h.clusters.Update(cluster)
}

// setResetCondition updates the ImportedDay2OpsReset condition only when the status payload actually changes.
func (h *handler) setResetCondition(cluster *apimgmtv3.Cluster, status, reason, message string) (*apimgmtv3.Cluster, error) {
	updated := cluster.DeepCopy()
	importedDay2OpsResetCondition.SetStatus(updated, status)
	importedDay2OpsResetCondition.Reason(updated, reason)
	importedDay2OpsResetCondition.Message(updated, message)

	if reflect.DeepEqual(cluster.Status.Conditions, updated.Status.Conditions) {
		return cluster, nil
	}
	return h.clusters.UpdateStatus(updated)
}

// resetBeaconWait returns the user-facing wait reason when another controller still owns the beacon.
func resetBeaconWait(beacon *planv1alpha1.Beacon) (bool, string) {
	if beacon == nil {
		return false, ""
	}
	owner := ""
	if beacon.Labels != nil {
		owner = beacon.Labels[planv1alpha1.BeaconOwnerLabel]
	}
	switch {
	case owner == resetBeaconOwnerKey:
		return false, ""
	case owner != "":
		return true, fmt.Sprintf("waiting for beacon release from %q", owner)
	default:
		return false, ""
	}
}

// shouldReconcileImportedDisable keeps reset reconciliation sticky once reset has started or disable is explicit.
func shouldReconcileImportedDisable(annotations map[string]string) bool {
	return annotations[importedDay2OpsResetInProgressAnnotation] == "true" || annotations[day2OpsEnabledAnnotation] == "false"
}

// SystemAgentUpgraderVersion returns the version of the system-agent-upgrader,
// which is determined by the image tag or defaults to "latest" if unspecified.
func SystemAgentUpgraderVersion() string {
	upgradeImage := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := "latest"
	if len(upgradeImage) == 2 {
		version = upgradeImage[1]
	}
	return version
}

// installer renders the imported system-agent SUC resources for the enabled state.
func installer(cluster *apimgmtv3.Cluster, secretName string) []runtime.Object {
	return buildSUCPlans(cluster, secretName, planEnv(cluster))
}

// uninstaller renders the imported system-agent SUC resources with DELETE=true for teardown.
func uninstaller(cluster *apimgmtv3.Cluster, secretName string) []runtime.Object {
	env := planEnv(cluster)
	foundDelete := false
	for i := range env {
		if env[i].Name == "DELETE" {
			env[i].Value = "true"
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		env = append(env, corev1.EnvVar{Name: "DELETE", Value: "true"})
	}
	return buildSUCPlans(cluster, secretName, env)
}

// planEnv builds the shared plan env for install and uninstall while preserving user-provided STRICT_VERIFY.
func planEnv(cluster *apimgmtv3.Cluster) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, e := range cluster.Spec.AgentEnvVars {
		env = append(env, corev1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		})
	}

	// Merge the env vars with the AgentTLSModeStrict
	found := false
	for _, ev := range env {
		if ev.Name == "STRICT_VERIFY" {
			found = true // The user has specified `STRICT_VERIFY`, we should not attempt to overwrite it.
		}
	}
	if !found {
		if settings.AgentTLSMode.Get() == settings.AgentTLSModeStrict {
			env = append(env, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "true",
			})
		} else {
			env = append(env, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "false",
			})
		}
	}
	return env
}

// buildSUCPlans renders the shared SUC plans and RBAC objects for imported system-agent management.
func buildSUCPlans(cluster *apimgmtv3.Cluster, secretName string, env []corev1.EnvVar) []runtime.Object {
	upgradeImage := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := SystemAgentUpgraderVersion()

	// todo: data directory detection
	var plans []runtime.Object

	plan := upgradev1.NewPlan(namespaces.System, SystemAgentUpgraderPlanName, upgradev1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				UpgradeDigestAnnotation: "spec.upgrade.envs,spec.upgrade.envFrom",
			},
		},
		Spec: upgradev1.PlanSpec{
			Concurrency: 10,
			Version:     version,
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      corev1.LabelOSStable,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"linux",
						},
					},
				},
			},
			ServiceAccountName: SystemAgentUpgraderServiceAccountName,
			// envFrom is still the source of CATTLE_ vars in plan, however secrets will trigger an update when changed.
			Secrets: []upgradev1.SecretSpec{
				{
					Name: "stv-aggregation",
				},
			},
			Upgrade: &upgradev1.ContainerSpec{
				Image: image.ResolveWithCluster(upgradeImage[0], cluster),
				Env:   env,
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				}},
			},
		},
	})
	plans = append(plans, plan)

	windowsPlan := winsUpgradePlan(cluster, env, secretName)

	// todo: redeploy support
	plans = append(plans, windowsPlan)

	objs := []runtime.Object{
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SystemAgentUpgraderServiceAccountName,
				Namespace: namespaces.System,
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: SystemAgentUpgraderClusterRoleName,
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"nodes"},
			}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: SystemAgentUpgraderClusterRoleBindingName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      SystemAgentUpgraderServiceAccountName,
				Namespace: namespaces.System,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     SystemAgentUpgraderClusterRoleName,
			},
		},
	}

	return append(plans, objs...)
}

func winsUpgradePlan(cluster *apimgmtv3.Cluster, env []corev1.EnvVar, secretName string) *upgradev1.Plan {
	winsUpgradeImage := strings.SplitN(settings.WinsAgentUpgradeImage.Get(), ":", 2)
	winsVersion := "latest"
	if len(winsUpgradeImage) == 2 {
		winsVersion = winsUpgradeImage[1]
	}

	return upgradev1.NewPlan(namespaces.System, SystemAgentUpgraderWindowsPlanName, upgradev1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SystemAgentUpgraderWindowsPlanName,
			Namespace: namespaces.System,
			Annotations: map[string]string{
				UpgradeDigestAnnotation: "spec.upgrade.envs,spec.upgrade.envFrom",
			},
		},
		Spec: upgradev1.PlanSpec{
			Concurrency: 10,
			Version:     winsVersion,
			Tolerations: []corev1.Toleration{
				{
					Operator: corev1.TolerationOpExists,
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      corev1.LabelOSStable,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"windows",
						},
					},
				},
			},
			ServiceAccountName: SystemAgentUpgraderServiceAccountName,
			Upgrade: &upgradev1.ContainerSpec{
				Image: image.ResolveWithCluster(winsUpgradeImage[0], cluster),
				Env:   env,
				SecurityContext: &corev1.SecurityContext{
					WindowsOptions: &corev1.WindowsSecurityContextOptions{
						HostProcess:   ptr.To(true),
						RunAsUserName: ptr.To("NT AUTHORITY\\SYSTEM"),
					},
				},
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				}},
			},
		},
	})
}
