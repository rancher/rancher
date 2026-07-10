package systemagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbaccontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

const (
	UpgradeDigestAnnotation                  = "upgrade.cattle.io/digest"
	AppliedSystemAgentUpgraderHashAnnotation = "management.cattle.io/applied-system-agent-upgrader-hash"
	day2OpsEnabledAnnotation                 = "operations.cattle.io/ops-enabled"
	importedCleaningStateAnnotation          = "operations.cattle.io/imported-cleaning-state"
	importedUninstallRolloutIDAnnotation     = "operations.cattle.io/imported-uninstall-rollout-id"

	systemAgentUpgraderRolloutIDLabel = "management.cattle.io/system-agent-upgrader-rollout-id"
	systemAgentUpgraderRunIDEnvName   = "SYSTEM_AGENT_UPGRADER_RUN_ID"

	SystemAgentUpgraderPlanName               = "system-agent-upgrader"
	SystemAgentUpgraderWindowsPlanName        = "system-agent-upgrader-windows"
	SystemAgentUpgraderServiceAccountName     = "system-agent-upgrader"
	SystemAgentUpgraderClusterRoleName        = "system-agent-upgrader"
	SystemAgentUpgraderClusterRoleBindingName = "system-agent-upgrader"
	systemAgentPlanEnvSecretName              = "stv-aggregation"
	disableBeaconOwnerKey                     = "imported-day2ops-disable"
	importedDay2OpsDisableRequeueInterval     = 5 * time.Second
)

var (
	// installCounter keeps track of the number of clusters for which the handler is concurrently installing or upgrading
	// the resources needed for upgrading system-agent.
	installCounter atomic.Int32

	upgradePlanGVR = schema.GroupVersionResource{Group: "upgrade.cattle.io", Version: "v1", Resource: "plans"}
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
	serviceAccounts          corecontrollers.ServiceAccountClient
	serviceAccountCache      corecontrollers.ServiceAccountCache
	secrets                  corecontrollers.SecretClient
	secretCache              corecontrollers.SecretCache
	roles                    rbaccontrollers.RoleClient
	roleBindings             rbaccontrollers.RoleBindingClient
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
		serviceAccounts:          w.Core.ServiceAccount(),
		serviceAccountCache:      w.Core.ServiceAccount().Cache(),
		secrets:                  w.Core.Secret(),
		secretCache:              w.Core.Secret().Cache(),
		roles:                    w.RBAC.Role(),
		roleBindings:             w.RBAC.RoleBinding(),
	}
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

	// Once imported disable has started, keep reconciling disable until it reaches a safe terminal
	// point, even if ops-enabled is flipped back to true.
	if shouldReconcileImportedDisable(cluster.Annotations) {
		return h.reconcileImportedDisable(cluster)
	}
	if cluster.Annotations[day2OpsEnabledAnnotation] != "true" {
		return cluster, nil
	}
	return h.reconcileImportedEnable(cluster)
}

// reconcileImportedEnable clears any disable bookkeeping before continuing the normal install path.
func (h *handler) reconcileImportedEnable(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	var err error
	cluster, changed, err := h.clearCleaningState(cluster)
	if err != nil {
		return cluster, err
	}
	if changed {
		return cluster, nil
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
		// skip further work if the downstream API server is not reachable,
		// this usually happen during cattle-cluster-agent being redeployed
		logrus.Debugf("[importedsystemagent] [%s] downstream API server is not reachable, will try again", cluster.Name)
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}

	result := installer(cluster)

	hash, err := renderedObjectsHash(result)
	if err != nil {
		return cluster, err
	}

	val, ok := cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation]
	identityExists, _, err := h.importedPlanIdentityExists(cluster.Name)
	if err != nil {
		return cluster, err
	}
	if ok && hash == val && identityExists {
		logrus.Debugf("[importedsystemagent] cluster %s/%s: applied templates for system-agent-upgrader is up to date. "+
			"To trigger a force redeployment, remove the %s annotation from the corresponding management cluster object",
			cluster.Namespace, cluster.Name, AppliedSystemAgentUpgraderHashAnnotation)
		return cluster, nil
	}
	if ok && hash == val && !identityExists {
		logrus.Infof("[importedsystemagent] cluster %s/%s: applied hash is current, but imported plan identity is incomplete; reapplying system-agent-upgrader templates", cluster.Namespace, cluster.Name)
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

	identityExists, waitMessage, err := h.importedPlanIdentityExists(cluster.Name)
	if err != nil {
		return cluster, err
	}
	if !identityExists {
		logrus.Debugf("[importedsystemagent] cluster %s/%s: %s", cluster.Namespace, cluster.Name, waitMessage)
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
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

// reconcileImportedDisable tears down imported day2ops in disable-safe order while holding the beacon for disable.
func (h *handler) reconcileImportedDisable(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	state := cluster.Annotations[importedCleaningStateAnnotation]
	if state == "" {
		needed, err := h.disableNeeded(cluster)
		if err != nil {
			return cluster, err
		}
		if !needed {
			return h.clearClusterAnnotations(cluster)
		}
		cluster, err = h.setCleaningState(cluster, apimgmtv3.ImportedDay2OpsCleaningStateOperations)
		if err != nil {
			return cluster, err
		}
		h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
		return cluster, nil
	}

	switch state {
	case apimgmtv3.ImportedDay2OpsCleaningStateOperations:
		beacon, err := h.beaconCache.Get(cluster.Name, cluster.Name)
		if apierrors.IsNotFound(err) {
			beacon = nil
		} else if err != nil {
			return cluster, err
		}

		// Disable waits for the current holder to finish instead of preempting it.
		if wait, message := disableBeaconWait(beacon); wait {
			logrus.Debugf("[importedsystemagent] cluster %s/%s: %s", cluster.Namespace, cluster.Name, message)
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}
		// Disable takes the beacon before deleting operation CRs to block new starts while cleanup runs.
		if beacon != nil {
			acquired, err := planapi.AcquireBeacon(beacon, h.beacons, disableBeaconOwnerKey)
			if err != nil {
				return cluster, err
			}
			if acquired == nil {
				h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
				return cluster, nil
			}
		}

		if remaining, err := h.deleteOperations(cluster.Name); err != nil {
			return cluster, err
		} else if remaining {
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		cluster, err = h.setCleaningState(cluster, apimgmtv3.ImportedDay2OpsCleaningStateUninstall)
		if err != nil {
			return cluster, err
		}
		h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
		return cluster, nil

	case apimgmtv3.ImportedDay2OpsCleaningStateUninstall:
		cluster, rolloutID, created, err := h.ensureUninstallRollout(cluster)
		if err != nil {
			return cluster, err
		}
		if created {
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
		if err != nil {
			return cluster, err
		}
		if err := healthsyncer.IsAPIUp(h.ctx, clusterCtx.K8sClient.CoreV1().Namespaces()); err != nil {
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		if err := h.applyUninstallPlans(cluster, clusterCtx, rolloutID); err != nil {
			return cluster, err
		}

		complete, message, err := h.uninstallComplete(clusterCtx, rolloutID)
		if err != nil {
			return cluster, err
		}
		if !complete {
			logrus.Debugf("[importedsystemagent] cluster %s/%s: %s", cluster.Namespace, cluster.Name, message)
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		cluster, err = h.setCleaningState(cluster, apimgmtv3.ImportedDay2OpsCleaningStateSUC)
		if err != nil {
			return cluster, err
		}
		h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
		return cluster, nil

	case apimgmtv3.ImportedDay2OpsCleaningStateSUC:
		clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
		if err != nil {
			return cluster, err
		}
		if remaining, err := h.deleteSUCResources(clusterCtx); err != nil {
			return cluster, err
		} else if remaining {
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		cluster, err = h.setCleaningState(cluster, apimgmtv3.ImportedDay2OpsCleaningStateMachinePlans)
		if err != nil {
			return cluster, err
		}
		h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
		return cluster, nil

	case apimgmtv3.ImportedDay2OpsCleaningStateMachinePlans:
		if remaining, err := h.deleteMachinePlans(cluster.Name); err != nil {
			return cluster, err
		} else if remaining {
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		cluster, err := h.setCleaningState(cluster, apimgmtv3.ImportedDay2OpsCleaningStateBeacon)
		if err != nil {
			return cluster, err
		}
		h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
		return cluster, nil

	case apimgmtv3.ImportedDay2OpsCleaningStateBeacon:
		if remaining, err := h.deleteBeacon(cluster.Name); err != nil {
			return cluster, err
		} else if remaining {
			h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
			return cluster, nil
		}

		return h.clearClusterAnnotations(cluster)

	default:
		cluster, err := h.setCleaningState(cluster, apimgmtv3.ImportedDay2OpsCleaningStateOperations)
		if err != nil {
			return cluster, err
		}
		h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
		return cluster, nil
	}
}

// disableNeeded returns true while imported day2ops resources still exist or disable bookkeeping is still present.
func (h *handler) disableNeeded(cluster *apimgmtv3.Cluster) (bool, error) {
	if cluster.Annotations[importedCleaningStateAnnotation] != "" ||
		cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation] != "" ||
		cluster.Annotations[importedUninstallRolloutIDAnnotation] != "" {
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
	if len(machinePlans) > 0 {
		return true, nil
	}

	serviceAccounts, err := h.importedPlanServiceAccounts(cluster.Name)
	if err != nil {
		return false, err
	}
	if len(serviceAccounts) > 0 {
		return true, nil
	}

	secrets, err := h.secretCache.List(cluster.Name, labels.Everything())
	if err != nil {
		return false, err
	}
	for i := range secrets {
		if secrets[i].Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		if strings.HasSuffix(secrets[i].Labels[serviceaccounttoken.ServiceAccountSecretLabel], "-machine-plan") {
			return true, nil
		}
	}

	roles, err := h.roles.List(cluster.Name, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for i := range roles.Items {
		if strings.HasSuffix(roles.Items[i].Name, "-machine-plan") {
			return true, nil
		}
	}

	roleBindings, err := h.roleBindings.List(cluster.Name, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for i := range roleBindings.Items {
		if !strings.HasSuffix(roleBindings.Items[i].Name, "-machine-plan") {
			continue
		}
		for j := range roleBindings.Items[i].Subjects {
			subject := roleBindings.Items[i].Subjects[j]
			if subject.Kind == "ServiceAccount" &&
				subject.Name == roleBindings.Items[i].Name &&
				(subject.Namespace == "" || subject.Namespace == cluster.Name) {
				return true, nil
			}
		}
	}

	return false, nil
}

// hasOperations reports whether any imported day2ops operation CRs still reference the cluster.
func (h *handler) hasOperations(clusterName string) (bool, error) {
	// Imported day2ops operation CRs are expected to live in the imported cluster namespace.
	saves, err := h.etcdSnapshotSaveCache.List(clusterName, labels.Everything())
	if err != nil {
		return false, err
	}
	if len(saves) > 0 {
		return true, nil
	}

	restores, err := h.etcdSnapshotRestoreCache.List(clusterName, labels.Everything())
	if err != nil {
		return false, err
	}
	if len(restores) > 0 {
		return true, nil
	}

	rotations, err := h.encryptionRotationCache.List(clusterName, labels.Everything())
	if err != nil {
		return false, err
	}
	return len(rotations) > 0, nil
}

// deleteOperations deletes imported operation CRs one-by-one and returns true while any are still present.
func (h *handler) deleteOperations(clusterName string) (bool, error) {
	remaining := false

	// Imported day2ops operation CRs are expected to live in the imported cluster namespace.
	saves, err := h.etcdSnapshotSaveCache.List(clusterName, labels.Everything())
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

	restores, err := h.etcdSnapshotRestoreCache.List(clusterName, labels.Everything())
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

	rotations, err := h.encryptionRotationCache.List(clusterName, labels.Everything())
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

// deleteMachinePlans deletes imported machine-plan secrets and the corresponding plan
// identities. It returns true while any of those resources are still present.
func (h *handler) deleteMachinePlans(clusterName string) (bool, error) {
	remaining := false

	secrets, err := h.machinePlanSecrets(clusterName)
	if err != nil {
		return false, err
	}
	for i := range secrets {
		remaining = true
		if secrets[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.secrets.Delete(secrets[i].Namespace, secrets[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	serviceAccounts, err := h.importedPlanServiceAccounts(clusterName)
	if err != nil {
		return false, err
	}
	for i := range serviceAccounts {
		remaining = true
		if err := h.deleteImportedPlanIdentity(serviceAccounts[i]); err != nil {
			return false, err
		}
	}

	return remaining, nil
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

// importedPlanServiceAccounts lists the imported machine-plan service accounts for the cluster namespace.
func (h *handler) importedPlanServiceAccounts(clusterName string) ([]*corev1.ServiceAccount, error) {
	return h.serviceAccountCache.List(clusterName, labels.SelectorFromSet(labels.Set{
		capr.RoleLabel:        capr.RolePlan,
		capr.ClusterNameLabel: clusterName,
	}))
}

// importedPlanIdentityExists checks only for imported machine-plan secret and plan service account presence.
// It does not prove the full identity set (token secret, role, rolebinding) is fully coherent yet.
func (h *handler) importedPlanIdentityExists(clusterName string) (bool, string, error) {
	machinePlans, err := h.machinePlanSecrets(clusterName)
	if err != nil {
		return false, "", err
	}
	if len(machinePlans) == 0 {
		return false, "waiting for imported machine-plan secret creation", nil
	}

	serviceAccounts, err := h.importedPlanServiceAccounts(clusterName)
	if err != nil {
		return false, "", err
	}
	if len(serviceAccounts) == 0 {
		return false, "waiting for imported machine-plan service account creation", nil
	}

	return true, "", nil
}

// deleteImportedPlanIdentity removes the imported machine-plan service account,
// token secret, role, and rolebinding associated with a plan identity.
func (h *handler) deleteImportedPlanIdentity(serviceAccount *corev1.ServiceAccount) error {
	tokenSecrets, err := h.secretCache.List(serviceAccount.Namespace, labels.SelectorFromSet(labels.Set{
		serviceaccounttoken.ServiceAccountSecretLabel: serviceAccount.Name,
	}))
	if err != nil {
		return err
	}
	for i := range tokenSecrets {
		if tokenSecrets[i].Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		if tokenSecrets[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.secrets.Delete(tokenSecrets[i].Namespace, tokenSecrets[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := h.roleBindings.Delete(serviceAccount.Namespace, serviceAccount.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := h.roles.Delete(serviceAccount.Namespace, serviceAccount.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if serviceAccount.DeletionTimestamp == nil {
		if err := h.serviceAccounts.Delete(serviceAccount.Namespace, serviceAccount.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
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

func (h *handler) ensureUninstallRollout(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, string, bool, error) {
	if cluster.Annotations[importedUninstallRolloutIDAnnotation] != "" {
		return cluster, cluster.Annotations[importedUninstallRolloutIDAnnotation], false, nil
	}

	rolloutID := string(uuid.NewUUID())

	updatedCluster := cluster.DeepCopy()
	if updatedCluster.Annotations == nil {
		updatedCluster.Annotations = map[string]string{}
	}
	updatedCluster.Annotations[importedUninstallRolloutIDAnnotation] = rolloutID

	updated, err := h.clusters.Update(updatedCluster)
	if err != nil {
		return cluster, "", false, err
	}
	return updated, rolloutID, true, nil
}

func renderedObjectsHash(objs []runtime.Object) (string, error) {
	data, err := json.Marshal(objs)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// applyUninstallPlans renders and applies the imported system-agent uninstall plans downstream.
func (h *handler) applyUninstallPlans(cluster *apimgmtv3.Cluster, clusterCtx *config.UserContext, rolloutID string) error {
	result := uninstaller(cluster, rolloutID)

	applier, err := apply.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return err
	}
	if err := applier.
		WithSetID("managed-system-agent").
		WithDynamicLookup().
		WithDefaultNamespace(namespaces.System).
		ApplyObjects(result...); err != nil {
		return err
	}

	return nil
}

// uninstallComplete waits for the rendered uninstall plans to exist, match the current
// rollout, and finish applying on every targeted node.
func (h *handler) uninstallComplete(clusterCtx *config.UserContext, rolloutID string) (bool, string, error) {
	for _, name := range []string{SystemAgentUpgraderPlanName, SystemAgentUpgraderWindowsPlanName} {
		plan, err := h.getDownstreamPlan(clusterCtx, name)
		if err != nil {
			return false, "", err
		}
		if plan == nil {
			return false, fmt.Sprintf("waiting for uninstall plan %s to be created", name), nil
		}
		if plan.Spec.Upgrade == nil {
			return false, fmt.Sprintf("waiting for uninstall plan %s to reconcile: upgrade spec is empty", name), nil
		}

		nodes, err := h.planTargetedNodes(clusterCtx, plan)
		if err != nil {
			return false, "", err
		}
		if ready, message := uninstallPlanReady(plan, rolloutID, nodes); !ready {
			return false, fmt.Sprintf("waiting for uninstall plan %s: %s", name, message), nil
		}
	}
	return true, "", nil
}

func uninstallPlanReady(plan *upgradev1.Plan, rolloutID string, nodes []corev1.Node) (bool, string) {
	uninstallValue, uninstallCount := envVarValue(plan.Spec.Upgrade.Env, "UNINSTALL")
	if uninstallCount != 1 || uninstallValue != "true" {
		return false, "UNINSTALL=true env not observed exactly once"
	}

	runIDValue, runIDCount := envVarValue(plan.Spec.Upgrade.Env, systemAgentUpgraderRunIDEnvName)
	if runIDCount != 1 || runIDValue != rolloutID {
		return false, fmt.Sprintf("%s=%s env not observed exactly once", systemAgentUpgraderRunIDEnvName, rolloutID)
	}

	if plan.Spec.PostCompleteLabels[systemAgentUpgraderRolloutIDLabel] != rolloutID {
		return false, fmt.Sprintf("postCompleteLabels.%s=%s not observed", systemAgentUpgraderRolloutIDLabel, rolloutID)
	}

	// Plan conditions alone are not a freshness proof because this controller reuses the
	// same SUC plan across install and uninstall. Require the per-node rollout receipt first.
	// The windows plan is rendered for all clusters, but Linux-only imported clusters have no
	// matching windows nodes. Skip wait gating for plans that currently target no nodes.
	if plan.Name == SystemAgentUpgraderWindowsPlanName && len(nodes) == 0 {
		return true, ""
	}
	if len(nodes) == 0 {
		return false, "no targeted nodes observed yet"
	}

	for i := range nodes {
		if nodes[i].Labels[systemAgentUpgraderRolloutIDLabel] != rolloutID {
			return false, fmt.Sprintf("node %s missing %s=%s", nodes[i].Name, systemAgentUpgraderRolloutIDLabel, rolloutID)
		}
	}

	if !upgradev1.PlanComplete.IsTrue(plan) {
		msg := upgradev1.PlanComplete.GetMessage(plan)
		if msg == "" {
			msg = upgradev1.PlanComplete.GetReason(plan)
		}
		if msg == "" {
			msg = "plan not complete yet"
		}
		return false, fmt.Sprintf("completion condition not met: %s", msg)
	}
	if len(plan.Status.Applying) > 0 {
		return false, fmt.Sprintf("still applying on nodes: %s", strings.Join(plan.Status.Applying, ","))
	}

	return true, ""
}

func envVarValue(env []corev1.EnvVar, name string) (string, int) {
	value := ""
	count := 0
	for i := range env {
		if env[i].Name != name {
			continue
		}
		count++
		value = env[i].Value
	}
	return value, count
}

func (h *handler) planTargetedNodes(clusterCtx *config.UserContext, planObj *upgradev1.Plan) ([]corev1.Node, error) {
	selector := labels.Everything()
	if planObj != nil && planObj.Spec.NodeSelector != nil {
		var err error
		selector, err = metav1.LabelSelectorAsSelector(planObj.Spec.NodeSelector)
		if err != nil {
			return nil, err
		}
	}

	hostnameRequirement, err := labels.NewRequirement(corev1.LabelHostname, selection.Exists, nil)
	if err != nil {
		return nil, err
	}
	selector = selector.Add(*hostnameRequirement)

	nodes, err := clusterCtx.K8sClient.CoreV1().Nodes().List(h.ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// getDownstreamPlan reads a downstream SUC plan through the dynamic client so disable can poll its status.
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
	for _, name := range []string{
		SystemAgentUpgraderPlanName,
		SystemAgentUpgraderWindowsPlanName,
	} {
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

// setCleaningState sets the current imported cleanup phase annotation.
func (h *handler) setCleaningState(cluster *apimgmtv3.Cluster, state string) (*apimgmtv3.Cluster, error) {
	if cluster.Annotations[importedCleaningStateAnnotation] == state {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[importedCleaningStateAnnotation] = state
	updated, err := h.clusters.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return updated, nil
}

// clearCleaningState removes the imported cleanup phase annotation.
func (h *handler) clearCleaningState(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, bool, error) {
	if cluster.Annotations[importedCleaningStateAnnotation] == "" {
		return cluster, false, nil
	}
	cluster = cluster.DeepCopy()
	delete(cluster.Annotations, importedCleaningStateAnnotation)
	updated, err := h.clusters.Update(cluster)
	if err != nil {
		return cluster, false, err
	}
	return updated, true, nil
}

// clearClusterAnnotations removes imported disable bookkeeping while preserving the user's ops-enabled choice.
func (h *handler) clearClusterAnnotations(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster.Annotations == nil {
		return cluster, nil
	}
	if cluster.Annotations[importedCleaningStateAnnotation] == "" &&
		cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation] == "" &&
		cluster.Annotations[importedUninstallRolloutIDAnnotation] == "" {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	delete(cluster.Annotations, importedCleaningStateAnnotation)
	delete(cluster.Annotations, AppliedSystemAgentUpgraderHashAnnotation)
	delete(cluster.Annotations, importedUninstallRolloutIDAnnotation)
	return h.clusters.Update(cluster)
}

// disableBeaconWait returns the user-facing wait reason when another controller still owns the beacon.
func disableBeaconWait(beacon *planv1alpha1.Beacon) (bool, string) {
	if beacon == nil {
		return false, ""
	}
	owner := ""
	if beacon.Labels != nil {
		owner = beacon.Labels[planv1alpha1.BeaconOwnerLabel]
	}
	switch {
	case owner == disableBeaconOwnerKey:
		return false, ""
	case owner != "":
		return true, fmt.Sprintf("waiting for beacon release from %q", owner)
	default:
		return false, ""
	}
}

// shouldReconcileImportedDisable keeps disable reconciliation sticky once disable has started or disable is explicit.
func shouldReconcileImportedDisable(annotations map[string]string) bool {
	return annotations[importedCleaningStateAnnotation] != "" || annotations[day2OpsEnabledAnnotation] == "false"
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
func installer(cluster *apimgmtv3.Cluster) []runtime.Object {
	plans := buildSUCPlanObjects(cluster, withUninstallEnv(planEnv(cluster), "false", ""), nil)
	return append(plans, sharedSUCObjects()...)
}

// uninstaller renders the imported system-agent SUC resources with UNINSTALL=true for teardown.
func uninstaller(cluster *apimgmtv3.Cluster, rolloutID string) []runtime.Object {
	plans := buildSUCPlanObjects(
		cluster,
		withUninstallEnv(planEnv(cluster), "true", rolloutID),
		map[string]string{systemAgentUpgraderRolloutIDLabel: rolloutID},
	)
	return append(plans, sharedSUCObjects()...)
}

// UNINSTALL must be first because SUC digest reconciliation has proven sensitive to env ordering.
func withUninstallEnv(env []corev1.EnvVar, uninstallValue, rolloutID string) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(env)+2)
	result = append(result, corev1.EnvVar{Name: "UNINSTALL", Value: uninstallValue})
	if rolloutID != "" {
		result = append(result, corev1.EnvVar{Name: systemAgentUpgraderRunIDEnvName, Value: rolloutID})
	}
	for _, entry := range env {
		if entry.Name == "UNINSTALL" || entry.Name == systemAgentUpgraderRunIDEnvName {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// planEnv builds the shared plan env for install and uninstall while preserving user-provided STRICT_VERIFY.
// Imported day2ops always forces CATTLE_ROLE_NONE=true.
func planEnv(cluster *apimgmtv3.Cluster) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, e := range cluster.Spec.AgentEnvVars {
		if e.Name == "CATTLE_ROLE_NONE" {
			continue
		}
		env = append(env, corev1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		})
	}

	// Merge the env vars with the AgentTLSModeStrict
	strictVerifyFound := false
	for _, ev := range env {
		if ev.Name == "STRICT_VERIFY" {
			strictVerifyFound = true // The user has specified `STRICT_VERIFY`, we should not attempt to overwrite it.
		}
	}
	if !strictVerifyFound {
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
	env = append(env, corev1.EnvVar{
		Name:  "CATTLE_ROLE_NONE",
		Value: "true",
	})
	return env
}

// buildSUCPlanObjects renders linux/windows SUC plans for imported system-agent management.
func buildSUCPlanObjects(cluster *apimgmtv3.Cluster, env []corev1.EnvVar, postCompleteLabels map[string]string) []runtime.Object {
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
					Name: systemAgentPlanEnvSecretName,
				},
			},
			Upgrade: &upgradev1.ContainerSpec{
				Image: image.ResolveWithCluster(upgradeImage[0], cluster),
				Env:   env,
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: systemAgentPlanEnvSecretName,
						},
					},
				}},
			},
			PostCompleteLabels: copyLabels(postCompleteLabels),
		},
	})
	plans = append(plans, plan)

	windowsPlan := winsUpgradePlan(cluster, env, postCompleteLabels)

	// todo: redeploy support
	plans = append(plans, windowsPlan)

	return plans
}

func copyLabels(labelMap map[string]string) map[string]string {
	if len(labelMap) == 0 {
		return nil
	}
	result := make(map[string]string, len(labelMap))
	for key, value := range labelMap {
		result[key] = value
	}
	return result
}

func sharedSUCObjects() []runtime.Object {
	return []runtime.Object{
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
}

func winsUpgradePlan(cluster *apimgmtv3.Cluster, env []corev1.EnvVar, postCompleteLabels map[string]string) *upgradev1.Plan {
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
							Name: systemAgentPlanEnvSecretName,
						},
					},
				}},
			},
			PostCompleteLabels: copyLabels(postCompleteLabels),
		},
	})
}
