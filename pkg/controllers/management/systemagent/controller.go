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

	lassodynamic "github.com/rancher/lasso/pkg/dynamic"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/clustermanager"
	k8sprovider "github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	UpgradeDigestAnnotation                  = "upgrade.cattle.io/digest"
	AppliedSystemAgentUpgraderHashAnnotation = "management.cattle.io/applied-system-agent-upgrader-hash"
	day2OpsEnabledAnnotation                 = "rancher.io/operations-enabled"
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

// OperationsEnabledForCluster checks if imported day2ops is enabled for a given cluster.
func OperationsEnabledForCluster(cluster *apimgmtv3.Cluster) bool {
	if cluster == nil {
		return false
	}

	value := ""
	if cluster.Annotations != nil {
		value = cluster.Annotations[day2OpsEnabledAnnotation]
	}

	switch value {
	case "true":
		return true
	case "false":
		return false
	case "system-default":
		fallthrough
	default:
		return settings.ImportedClusterDay2OpsEnabledDefault.Get() == "true"
	}
}

var (
	// installCounter keeps track of the number of clusters for which the handler is concurrently installing or upgrading
	// the resources needed for upgrading system-agent.
	installCounter atomic.Int32

	upgradePlanGVR = schema.GroupVersionResource{Group: "upgrade.cattle.io", Version: "v1", Resource: "plans"}
)

type handler struct {
	ctx     context.Context
	manager *clustermanager.Manager
	dynamic *lassodynamic.Controller

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
	roleCache                rbaccontrollers.RoleCache
	roleBindings             rbaccontrollers.RoleBindingClient
	roleBindingCache         rbaccontrollers.RoleBindingCache
}

func Register(ctx context.Context, w *wrangler.Context, manager *clustermanager.Manager) {
	h := &handler{
		ctx:                      ctx,
		manager:                  manager,
		dynamic:                  w.Dynamic,
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
		roleCache:                w.RBAC.Role().Cache(),
		roleBindings:             w.RBAC.RoleBinding(),
		roleBindingCache:         w.RBAC.RoleBinding().Cache(),
	}
	w.Mgmt.Cluster().OnChange(ctx, "imported-system-agent-setup", h.onChange)
}

// shouldInstall matches the historical scope of this controller: imported RKE2/K3s and
// imported CAPR-backed RKE2/K3s clusters, while skipping provisioned/administrated clusters.
func shouldInstall(cluster *apimgmtv3.Cluster) bool {
	if cluster == nil {
		return false
	}

	if cluster.Name == "local" {
		return false
	}

	if cluster.Annotations != nil && cluster.Annotations["provisioning.cattle.io/administrated"] == "true" {
		return false
	}

	if cluster.Status.Driver == apimgmtv3.ClusterDriverK3s {
		return true
	}
	if cluster.Status.Driver == apimgmtv3.ClusterDriverRke2 {
		return true
	}

	if cluster.Labels != nil && cluster.Labels[k8sprovider.ProviderKey] == "rke2" {
		return true
	}
	if cluster.Labels != nil && cluster.Labels[k8sprovider.ProviderKey] == "k3s" {
		return true
	}

	return false
}

func (h *handler) clusterOwner(cluster *apimgmtv3.Cluster) (*corev1.ObjectReference, error) {
	if cluster.Labels != nil &&
		cluster.Labels["cluster-api.cattle.io/capi-cluster-owner"] != "" &&
		cluster.Labels["cluster-api.cattle.io/capi-cluster-owner-ns"] != "" {
		capiCluster, err := h.dynamic.Get(
			capiv1beta2.GroupVersion.WithKind("Cluster"),
			cluster.Labels["cluster-api.cattle.io/capi-cluster-owner-ns"],
			cluster.Labels["cluster-api.cattle.io/capi-cluster-owner"],
		)
		if err != nil {
			return nil, err
		}
		m, err := meta.Accessor(capiCluster)
		if err != nil {
			return nil, err
		}
		return &corev1.ObjectReference{
			APIVersion: capiv1beta2.GroupVersion.String(),
			Kind:       "Cluster",
			Namespace:  m.GetNamespace(),
			Name:       m.GetName(),
			UID:        m.GetUID(),
		}, nil
	}

	return &corev1.ObjectReference{
		APIVersion: cluster.APIVersion,
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}, nil
}

func (h *handler) onChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if !shouldInstall(cluster) {
		return cluster, nil
	}

	enabled := features.ImportedDay2Ops.Enabled() && OperationsEnabledForCluster(cluster)

	// Once imported disable has started, keep reconciling disable until it reaches a safe terminal
	// point, even if ops-enabled is flipped back to true.
	if shouldReconcileImportedDisable(cluster.Annotations) {
		return h.reconcileImportedDisable(cluster)
	}

	if !enabled {
		return h.reconcileImportedDisable(cluster)
	}

	return h.reconcileImportedEnable(cluster)
}

// reconcileImportedEnable clears any disable bookkeeping before continuing the normal install path.
func (h *handler) reconcileImportedEnable(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster.Annotations[importedCleaningStateAnnotation] != "" {
		cluster = cluster.DeepCopy()
		delete(cluster.Annotations, importedCleaningStateAnnotation)
		updated, err := h.clusters.Update(cluster)
		if err != nil {
			return cluster, err
		}
		return updated, nil
	}

	return h.reconcileImportedInstall(cluster)
}

// reconcileImportedInstall ensures the imported system-agent upgrade resources exist and match the current template hash.
func (h *handler) reconcileImportedInstall(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	ref, err := h.clusterOwner(cluster)
	if err != nil {
		return nil, err
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = ref.Name
	}

	_, err = h.beaconCache.Get(namespace, ref.Name)
	if apierrors.IsNotFound(err) {
		_, err = h.beacons.Create(&planv1alpha1.Beacon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: ref.APIVersion,
						Kind:       ref.Kind,
						Name:       ref.Name,
						UID:        ref.UID,
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

	data, err := json.Marshal(result)
	if err != nil {
		return cluster, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	val, ok := cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation]
	identityState, err := h.importedPlanIdentity(cluster.Name)
	if err != nil {
		return cluster, err
	}
	waitMessage := identityState.waitMessage()
	if ok && hash == val && waitMessage == "" {
		logrus.Debugf("[importedsystemagent] cluster %s/%s: applied templates for system-agent-upgrader is up to date. "+
			"To trigger a force redeployment, remove the %s annotation from the corresponding management cluster object",
			cluster.Namespace, cluster.Name, AppliedSystemAgentUpgraderHashAnnotation)
		return cluster, nil
	}
	if ok && hash == val && waitMessage != "" {
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

	identityState, err = h.importedPlanIdentity(cluster.Name)
	if err != nil {
		return cluster, err
	}
	waitMessage = identityState.waitMessage()
	if waitMessage != "" {
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
		ref, err := h.clusterOwner(cluster)
		if err != nil {
			return nil, err
		}
		namespace := ref.Namespace
		if namespace == "" {
			namespace = ref.Name
		}

		// First block new day2ops work by taking the beacon, then remove any in-flight
		// operation CRs before starting the system-agent uninstall rollout.
		beacon, err := h.beaconCache.Get(namespace, ref.Name)
		if apierrors.IsNotFound(err) {
			beacon = nil
		} else if err != nil {
			return cluster, err
		}

		// Disable waits for the current holder to finish instead of preempting it.
		if beacon != nil {
			owner := beacon.Status.Owner
			if owner != "" && owner != disableBeaconOwnerKey {
				logrus.Debugf("[importedsystemagent] cluster %s/%s: waiting for beacon release from %q", cluster.Namespace, cluster.Name, owner)
				h.clusters.EnqueueAfter(cluster.Name, importedDay2OpsDisableRequeueInterval)
				return cluster, nil
			}
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
		// Render and apply the uninstall plan, then wait for the current rollout ID to be
		// acknowledged on every targeted node before tearing down downstream SUC resources.
		rolloutID := cluster.Annotations[importedUninstallRolloutIDAnnotation]
		if rolloutID == "" {
			rolloutID = string(uuid.NewUUID())

			cluster = cluster.DeepCopy()
			if cluster.Annotations == nil {
				cluster.Annotations = map[string]string{}
			}
			cluster.Annotations[importedUninstallRolloutIDAnnotation] = rolloutID

			updated, err := h.clusters.Update(cluster)
			if err != nil {
				return cluster, err
			}
			h.clusters.EnqueueAfter(updated.Name, importedDay2OpsDisableRequeueInterval)
			return updated, nil
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
		// Once uninstall has completed on the targeted nodes, remove the downstream SUC plans
		// and shared RBAC objects that were used to deliver the rollout.
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
		// After SUC is gone, delete the imported machine-plan secrets and their associated
		// service account identity so re-enable starts from a clean imported plan state.
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
		// Release the imported beacon last so new operations cannot start until all imported
		// day2ops bookkeeping and delivery resources have been removed.
		if remaining, err := h.deleteBeacon(cluster); err != nil {
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

	ref, err := h.clusterOwner(cluster)
	if err != nil {
		return false, err
	}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = ref.Name
	}

	_, err = h.beaconCache.Get(namespace, ref.Name)
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

	identityState, err := h.importedPlanIdentity(cluster.Name)
	if err != nil {
		return false, err
	}
	if identityState.exists() {
		return true, nil
	}

	return h.hasImportedPlanIdentityResources(cluster.Name)
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

	identityState, err := h.importedPlanIdentity(clusterName)
	if err != nil {
		return false, err
	}
	for i := range identityState.machinePlanSecrets {
		remaining = true
		if identityState.machinePlanSecrets[i].DeletionTimestamp != nil {
			continue
		}
		if err := h.secrets.Delete(identityState.machinePlanSecrets[i].Namespace, identityState.machinePlanSecrets[i].Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	for i := range identityState.serviceAccounts {
		remaining = true
		if err := h.deleteImportedPlanIdentity(identityState.serviceAccounts[i]); err != nil {
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

type importedPlanIdentityState struct {
	machinePlanSecrets []*corev1.Secret
	serviceAccounts    []*corev1.ServiceAccount
}

func (s importedPlanIdentityState) exists() bool {
	return len(s.machinePlanSecrets) > 0 || len(s.serviceAccounts) > 0
}

func (s importedPlanIdentityState) waitMessage() string {
	if len(s.machinePlanSecrets) == 0 {
		return "waiting for imported machine-plan secret creation"
	}
	if len(s.serviceAccounts) == 0 {
		return "waiting for imported machine-plan service account creation"
	}
	return ""
}

func (h *handler) importedPlanIdentity(clusterName string) (importedPlanIdentityState, error) {
	machinePlans, err := h.machinePlanSecrets(clusterName)
	if err != nil {
		return importedPlanIdentityState{}, err
	}

	serviceAccounts, err := h.importedPlanServiceAccounts(clusterName)
	if err != nil {
		return importedPlanIdentityState{}, err
	}

	return importedPlanIdentityState{
		machinePlanSecrets: machinePlans,
		serviceAccounts:    serviceAccounts,
	}, nil
}

func (h *handler) hasImportedPlanIdentityResources(clusterName string) (bool, error) {
	secrets, err := h.secretCache.List(clusterName, labels.Everything())
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

	roles, err := h.roleCache.List(clusterName, labels.Everything())
	if err != nil {
		return false, err
	}
	for i := range roles {
		if strings.HasSuffix(roles[i].Name, "-machine-plan") {
			return true, nil
		}
	}

	roleBindings, err := h.roleBindingCache.List(clusterName, labels.Everything())
	if err != nil {
		return false, err
	}
	for i := range roleBindings {
		if !strings.HasSuffix(roleBindings[i].Name, "-machine-plan") {
			continue
		}
		for j := range roleBindings[i].Subjects {
			subject := roleBindings[i].Subjects[j]
			if subject.Kind == "ServiceAccount" &&
				subject.Name == roleBindings[i].Name &&
				(subject.Namespace == "" || subject.Namespace == clusterName) {
				return true, nil
			}
		}
	}

	return false, nil
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
		if tokenSecrets[i].Type != corev1.SecretTypeServiceAccountToken || tokenSecrets[i].DeletionTimestamp != nil {
			continue
		}
		if tokenSecrets[i].Labels[serviceaccounttoken.ServiceAccountSecretLabel] != serviceAccount.Name {
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
func (h *handler) deleteBeacon(cluster *apimgmtv3.Cluster) (bool, error) {
	ref, err := h.clusterOwner(cluster)
	if err != nil {
		return false, err
	}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = ref.Name
	}

	beacon, err := h.beacons.Get(namespace, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if beacon.DeletionTimestamp == nil {
		if err := h.beacons.Delete(namespace, ref.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return true, nil
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
	env := installPlanEnv(planEnv(cluster))
	return append([]runtime.Object{
		linuxUpgradePlan(cluster, env),
		winsUpgradePlan(cluster, env),
	}, sharedSUCObjects()...)
}

// uninstaller renders the imported system-agent SUC resources with UNINSTALL=true for teardown.
func uninstaller(cluster *apimgmtv3.Cluster, rolloutID string) []runtime.Object {
	env := uninstallPlanEnv(planEnv(cluster), rolloutID)
	linuxPlan := linuxUpgradePlan(cluster, env)
	windowsPlan := winsUpgradePlan(cluster, env)

	// Uninstall rollout tracking is only needed for the teardown path.
	linuxPlan.Spec.PostCompleteLabels = map[string]string{systemAgentUpgraderRolloutIDLabel: rolloutID}
	windowsPlan.Spec.PostCompleteLabels = map[string]string{systemAgentUpgraderRolloutIDLabel: rolloutID}

	return append([]runtime.Object{linuxPlan, windowsPlan}, sharedSUCObjects()...)
}

// UNINSTALL must be first because SUC digest reconciliation has proven sensitive to env ordering.
func installPlanEnv(base []corev1.EnvVar) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(base)+1)
	result = append(result, corev1.EnvVar{Name: "UNINSTALL", Value: "false"})
	for _, entry := range base {
		if entry.Name == "UNINSTALL" || entry.Name == systemAgentUpgraderRunIDEnvName {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func uninstallPlanEnv(base []corev1.EnvVar, rolloutID string) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(base)+2)
	result = append(result, corev1.EnvVar{Name: "UNINSTALL", Value: "true"})
	result = append(result, corev1.EnvVar{Name: systemAgentUpgraderRunIDEnvName, Value: rolloutID})
	for _, entry := range base {
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

func linuxUpgradePlan(cluster *apimgmtv3.Cluster, env []corev1.EnvVar) *upgradev1.Plan {
	upgradeImage := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := SystemAgentUpgraderVersion()

	return upgradev1.NewPlan(namespaces.System, SystemAgentUpgraderPlanName, upgradev1.Plan{
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
		},
	})
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

func winsUpgradePlan(cluster *apimgmtv3.Cluster, env []corev1.EnvVar) *upgradev1.Plan {
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
		},
	})
}
