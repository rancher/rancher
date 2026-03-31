package globalroles

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/status"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const (
	globalRoleLabel       = "authz.management.cattle.io/globalrole"
	crNameAnnotation      = "authz.management.cattle.io/cr-name"
	initialSyncAnnotation = "authz.management.cattle.io/initial-sync"
	clusterRoleKind       = "ClusterRole"
	// grOwnerLabel is used to label ClusterRoles and Roles created by the GlobalRole with the name of the owning GlobalRole.
	grOwnerLabel = "authz.management.cattle.io/gr-owner"
)

// Condition reason types
const (
	ClusterRoleExists                 = "ClusterRoleExists"
	NamespacedRuleRoleExists          = "NamespacedRuleRoleExists"
	InheritedNamespacedRuleRoleExists = "InheritedNamespacedRuleRoleExists"
	NamespaceNotAvailable             = "NamespaceNotAvailable"
	FailedToGetNamespace              = "GetNamespaceFailed"
	FailedToCreateClusterRole         = "CreateClusterRoleFailed"
	FailedToUpdateClusterRole         = "UpdateClusterRoleFailed"
	FailedToGetCluster                = "GetClusterFailed"
)

type fleetPermissionsRoleHandler interface {
	reconcileFleetWorkspacePermissions(globalRole *v3.GlobalRole, localConditions *[]metav1.Condition) error
}

func newGlobalRoleLifecycle(management *config.ManagementContext, clusterManager *clustermanager.Manager) *globalRoleLifecycle {
	return &globalRoleLifecycle{
		clusters:                management.Wrangler.Mgmt.Cluster(),
		clusterManager:          clusterManager,
		crLister:                management.Wrangler.RBAC.ClusterRole().Cache(),
		crClient:                management.Wrangler.RBAC.ClusterRole(),
		nsCache:                 management.Wrangler.Core.Namespace().Cache(),
		rLister:                 management.Wrangler.RBAC.Role().Cache(),
		rClient:                 management.Wrangler.RBAC.Role(),
		grClient:                management.Wrangler.Mgmt.GlobalRole(),
		grCache:                 management.Wrangler.Mgmt.GlobalRole().Cache(),
		grbCache:                management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		fleetPermissionsHandler: newFleetWorkspaceRoleHandler(management),
		status:                  status.NewStatus(),
	}
}

type globalRoleLifecycle struct {
	clusters                mgmtv3.ClusterClient
	clusterManager          *clustermanager.Manager
	crLister                wrbacv1.ClusterRoleCache
	crClient                wrbacv1.ClusterRoleClient
	nsCache                 wcorev1.NamespaceCache
	rLister                 wrbacv1.RoleCache
	rClient                 wrbacv1.RoleClient
	grClient                mgmtv3.GlobalRoleClient
	grCache                 mgmtv3.GlobalRoleCache
	grbCache                mgmtv3.GlobalRoleBindingCache
	fleetPermissionsHandler fleetPermissionsRoleHandler
	status                  *status.Status
}

func (gr *globalRoleLifecycle) Create(obj *v3.GlobalRole) (runtime.Object, error) {
	localConditions := []metav1.Condition{}
	returnError := errors.Join(
		gr.reconcileGlobalRole(obj, &localConditions),
		gr.reconcileNamespacedRoles(obj, &localConditions),
		gr.reconcileInheritedNamespacedRoles(obj, &localConditions),
		gr.fleetPermissionsHandler.reconcileFleetWorkspacePermissions(obj, &localConditions),
		gr.updateStatus(obj, localConditions),
	)
	return obj, returnError
}

func (gr *globalRoleLifecycle) Updated(obj *v3.GlobalRole) (runtime.Object, error) {
	localConditions := []metav1.Condition{}
	returnError := errors.Join(
		gr.reconcileGlobalRole(obj, &localConditions),
		gr.reconcileNamespacedRoles(obj, &localConditions),
		gr.reconcileInheritedNamespacedRoles(obj, &localConditions),
		gr.fleetPermissionsHandler.reconcileFleetWorkspacePermissions(obj, &localConditions),
		gr.updateStatus(obj, localConditions),
	)
	return nil, returnError
}

func (gr *globalRoleLifecycle) Remove(obj *v3.GlobalRole) (runtime.Object, error) {
	if rbac.IsAdminGlobalRole(obj) {
		// List globalrolebindings that reference this global role.
		grbs, err := gr.grbCache.List(labels.Everything())
		if err != nil {
			return nil, err
		}

		var errs error
		for _, grb := range grbs {
			if grb.GlobalRoleName == obj.Name {
				err = DeleteAdminClusterRoleBindings(gr.clusters, gr.clusterManager, grb)
				if err != nil {
					errs = errors.Join(errs, fmt.Errorf("failed to delete ClusterRoleBindings for admin GlobalRoleBinding %s: %w", grb.Name, err))
				}
			}
		}
		if errs != nil {
			return nil, errs
		}
	}

	// Don't need to delete the created ClusterRole or Roles in local cluster because owner reference will take care of them
	// However, roles in downstream clusters need to be deleted manually
	err := gr.deleteInheritedNamespacedRoles(obj)
	return nil, err
}

func (gr *globalRoleLifecycle) reconcileGlobalRole(globalRole *v3.GlobalRole, localConditions *[]metav1.Condition) error {
	crName := getCRName(globalRole)
	condition := metav1.Condition{
		Type: ClusterRoleExists,
	}

	clusterRole, _ := gr.crLister.Get(crName)
	if clusterRole != nil {
		updated := false
		clusterRole = clusterRole.DeepCopy()
		if !reflect.DeepEqual(globalRole.Rules, clusterRole.Rules) {
			clusterRole.Rules = globalRole.Rules
			logrus.Infof("[%v] Updating clusterRole %v. GlobalRole rules have changed. Have: %+v. Want: %+v", grController, clusterRole.Name, clusterRole.Rules, globalRole.Rules)
			updated = true
		}
		// Ensure existing ClusterRoles have the correct grOwnerLabel pointing to the owning GlobalRole.
		if grName := clusterRole.Labels[grOwnerLabel]; grName != globalRole.Name {
			clusterRole.Labels[grOwnerLabel] = globalRole.Name
			logrus.Infof("[%v] Updating clusterRole %s owner from %s to %s.", grController, clusterRole.Name, grName, globalRole.Name)
			updated = true
		}

		if updated {
			if _, err := gr.crClient.Update(clusterRole); err != nil {
				gr.status.AddCondition(localConditions, condition, FailedToUpdateClusterRole, err)
				return fmt.Errorf("couldn't update ClusterRole %v: %w", clusterRole.Name, err)
			}
		}
		gr.status.AddCondition(localConditions, condition, ClusterRoleExists, nil)
		return nil
	}

	logrus.Infof("[%s] Creating clusterRole %s for corresponding GlobalRole", grController, crName)
	_, err := gr.crClient.Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: globalRole.TypeMeta.APIVersion,
					Kind:       globalRole.TypeMeta.Kind,
					Name:       globalRole.Name,
					UID:        globalRole.UID,
				},
			},
			Labels: map[string]string{
				globalRoleLabel: "true",
				grOwnerLabel:    globalRole.Name,
			},
		},
		Rules: globalRole.Rules,
	})
	if err != nil {
		gr.status.AddCondition(localConditions, condition, FailedToCreateClusterRole, err)
		return err
	}
	// Add an annotation to the globalrole indicating the name we used for future updates
	if globalRole.Annotations == nil {
		globalRole.Annotations = map[string]string{}
	}
	globalRole.Annotations[crNameAnnotation] = crName
	gr.status.AddCondition(localConditions, condition, ClusterRoleExists, nil)
	return nil
}

// reconcileNamespacedRoles ensures that Roles exist in each namespace of NamespacedRules
func (gr *globalRoleLifecycle) reconcileNamespacedRoles(globalRole *v3.GlobalRole, localConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: NamespacedRuleRoleExists}
	var returnError error
	globalRoleName := name.SafeConcatName(globalRole.Name)

	// For collecting all the roles that should exist for the GlobalRole
	roleUIDs := map[types.UID]struct{}{}

	for ns, rules := range globalRole.NamespacedRules {
		roleName := name.SafeConcatName(globalRole.Name, ns)

		shouldSkip, err := validateNamespace(gr.nsCache, ns, "local cluster")
		if shouldSkip {
			continue
		}
		if err != nil {
			returnError = errors.Join(returnError, err)
			continue
		}

		newRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: globalRole.APIVersion,
						Kind:       globalRole.Kind,
						Name:       globalRole.Name,
						UID:        globalRole.UID,
					},
				},
				Labels: map[string]string{
					grOwnerLabel: globalRoleName,
				},
			},
			Rules: rules,
		}

		role, err := rbac.CreateOrUpdateNamespacedResource(newRole, gr.rClient, areRolesEqual)
		if role != nil && err == nil {
			roleUIDs[role.GetUID()] = struct{}{}
		}
		returnError = errors.Join(returnError, err)
	}

	// get all the roles claiming to be owned by this GR and remove any that shouldn't exist
	selector, err := createOwnerLabelSelector(globalRoleName)
	if err != nil {
		return errors.Join(returnError, err)
	}

	roles, err := gr.rLister.List("", selector)
	if err != nil {
		returnError = errors.Join(returnError, fmt.Errorf("couldn't list roles with label %s: %s: %w", grOwnerLabel, globalRoleName, err))
		gr.status.AddCondition(localConditions, condition, NamespacedRuleRoleExists, returnError)
		return returnError
	}

	// After creating/updating all Roles, if the number of roles with the grOwnerLabel is the same as
	// the number of created/updated Roles, we know there are no invalid Roles to purge
	if len(roleUIDs) != len(roles) {
		err = deleteRolesByUID(roles, roleUIDs, gr.rClient)
		if err != nil {
			returnError = errors.Join(returnError, err)
		}
	}

	gr.status.AddCondition(localConditions, condition, NamespacedRuleRoleExists, returnError)
	return returnError
}

// reconcileInheritedNamespacedRoles ensures that Roles exist in each namespace of InheritedNamespacedRules for all downstream clusters
func (gr *globalRoleLifecycle) reconcileInheritedNamespacedRoles(globalRole *v3.GlobalRole, localConditions *[]metav1.Condition) error {
	// If there are no InheritedNamespacedRules, nothing to do
	if len(globalRole.InheritedNamespacedRules) == 0 {
		return nil
	}

	var returnError error

	// Get all clusters
	clusters, err := gr.clusters.List(metav1.ListOptions{})
	if err != nil {
		condition := metav1.Condition{
			Type: InheritedNamespacedRuleRoleExists,
		}
		gr.status.AddCondition(localConditions, condition, FailedToGetCluster, err)
		return fmt.Errorf("couldn't list clusters: %w", err)
	}

	// Iterate through all clusters except local
	for _, cluster := range clusters.Items {
		if cluster.Name == localClusterName {
			continue
		}

		clusterErr := gr.reconcileInheritedNamespacedRolesForCluster(&cluster, globalRole)
		if clusterErr != nil {
			returnError = errors.Join(returnError, clusterErr)
		}
	}

	// Add a single condition for all InheritedNamespacedRules
	condition := metav1.Condition{
		Type: InheritedNamespacedRuleRoleExists,
	}
	gr.status.AddCondition(localConditions, condition, InheritedNamespacedRuleRoleExists, returnError)
	return returnError
}

// reconcileInheritedNamespacedRolesForCluster reconciles roles for a single downstream cluster
func (gr *globalRoleLifecycle) reconcileInheritedNamespacedRolesForCluster(cluster *v3.Cluster, globalRole *v3.GlobalRole) error {
	// Get user context for the cluster
	userContext, err := gr.clusterManager.UserContext(cluster.Name)
	if err != nil {
		logrus.Warnf("[%v] Failed to get user context for cluster %s: %v. Continuing with other clusters.", grController, cluster.Name, err)
		return nil
	}

	// Get the Role client for this cluster
	roleClient := userContext.RBACw.Role()
	roleCache := roleClient.Cache()
	namespaceCache := userContext.Corew.Namespace().Cache()

	var returnError error

	// Collect the UIDs of all the roles that should exist for this cluster based on the InheritedNamespacedRules. This will be used later to purge any invalid roles that may exist.
	roleUIDs := map[types.UID]struct{}{}

	// Iterate through all namespaces in InheritedNamespacedRules
	for ns, rules := range globalRole.InheritedNamespacedRules {
		roleUID, nsErr := gr.reconcileInheritedRoleInNamespace(cluster.Name, ns, rules, globalRole.Name, roleClient, namespaceCache)
		if nsErr != nil {
			returnError = errors.Join(returnError, nsErr)
		}
		roleUIDs[roleUID] = struct{}{}
	}

	// Purge invalid roles in this cluster
	purgeErr := gr.purgeInvalidInheritedRolesInCluster(cluster.Name, globalRole.Name, roleCache, roleClient, roleUIDs)
	if purgeErr != nil {
		returnError = errors.Join(returnError, purgeErr)
	}

	return returnError
}

// reconcileInheritedRoleInNamespace reconciles a single role in a specific namespace of a downstream cluster
func (gr *globalRoleLifecycle) reconcileInheritedRoleInNamespace(clusterName, ns string, rules []rbacv1.PolicyRule, globalRoleName string, roleClient wrbacv1.RoleClient, namespaceCache wcorev1.NamespaceCache) (types.UID, error) {
	roleName := name.SafeConcatName(globalRoleName, ns)
	safeGlobalRoleName := name.SafeConcatName(globalRoleName)

	// Check if the namespace exists in this cluster
	shouldSkip, err := validateNamespace(namespaceCache, ns, fmt.Sprintf("cluster %s", clusterName))
	if shouldSkip {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("couldn't validate namespace %s in cluster %s: %w", ns, clusterName, err)
	}

	newRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
			Labels: map[string]string{
				grOwnerLabel: safeGlobalRoleName,
			},
		},
		Rules: rules,
	}

	role, err := rbac.CreateOrUpdateNamespacedResource(newRole, roleClient, areRolesEqual)
	if role == nil {
		return "", err
	}

	return role.UID, err
}

// purgeInvalidInheritedRolesInCluster removes roles in a cluster that are no longer needed
func (gr *globalRoleLifecycle) purgeInvalidInheritedRolesInCluster(clusterName, globalRoleName string, roleCache wrbacv1.RoleCache, roleClient wrbacv1.RoleClient, validRoleUIDs map[types.UID]struct{}) error {
	selector, err := createOwnerLabelSelector(name.SafeConcatName(globalRoleName))
	if err != nil {
		return fmt.Errorf("failed to create label selector for cluster %s: %w", clusterName, err)
	}

	roles, err := roleCache.List("", selector)
	if err != nil {
		return fmt.Errorf("couldn't list roles in cluster %s: %w", clusterName, err)
	}

	return deleteRolesByUID(roles, validRoleUIDs, roleClient)
}

// deleteInheritedNamespacedRoles removes all Roles in downstream clusters that are owned by this GlobalRole
func (gr *globalRoleLifecycle) deleteInheritedNamespacedRoles(globalRole *v3.GlobalRole) error {
	// If there are no InheritedNamespacedRules, nothing to do
	if len(globalRole.InheritedNamespacedRules) == 0 {
		return nil
	}

	var returnError error
	globalRoleName := name.SafeConcatName(globalRole.Name)
	selector, err := createOwnerLabelSelector(globalRoleName)
	if err != nil {
		return fmt.Errorf("couldn't create label selector: %w", err)
	}

	// Get all clusters
	clusters, err := gr.clusters.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't list clusters: %w", err)
	}

	// Iterate through all clusters except local
	for _, cluster := range clusters.Items {
		if cluster.Name == localClusterName {
			continue
		}

		// Get user context for the cluster
		userContext, err := gr.clusterManager.UserContext(cluster.Name)
		if err != nil {
			logrus.Warnf("[%v] Failed to get user context for cluster %s during cleanup: %v. Continuing with other clusters.", grController, cluster.Name, err)
			continue
		}

		// Get the Role client for this cluster
		roleClient := userContext.RBACw.Role()
		roleCache := roleClient.Cache()

		roles, err := roleCache.List("", selector)
		if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("couldn't list roles in cluster %s: %w", cluster.Name, err))
			continue
		}

		// Delete all roles owned by this GlobalRole
		for _, role := range roles {
			if err := rbac.DeleteNamespacedResource(role.Namespace, role.Name, roleClient); err != nil {
				returnError = errors.Join(returnError, fmt.Errorf("couldn't delete role %s in namespace %s in cluster %s: %w", role.Name, role.Namespace, cluster.Name, err))
			}
		}
	}

	return returnError
}

// updateStatus updates the Status field of the GlobalRole. localConditions are created in each reconciliation loop.
// Status is only updated if any condition has changed.
func (gr *globalRoleLifecycle) updateStatus(globalRole *v3.GlobalRole, localConditions []metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		grFromCluster, err := gr.grCache.Get(globalRole.Name)
		if err != nil {
			return err
		}
		if status.CompareConditions(grFromCluster.Status.Conditions, localConditions) {
			return nil
		}

		grCopy := grFromCluster.DeepCopy()
		grCopy.Status.Summary = status.SummaryCompleted
		for _, c := range localConditions {
			if c.Status != metav1.ConditionTrue {
				grCopy.Status.Summary = status.SummaryError
				break
			}
		}

		status.KeepLastTransitionTimeIfConditionHasNotChanged(localConditions, grFromCluster.Status.Conditions)
		grCopy.Status.LastUpdate = gr.status.TimeNow().Format(time.RFC3339)
		grCopy.Status.ObservedGeneration = globalRole.ObjectMeta.Generation
		grCopy.Status.Conditions = localConditions
		_, err = gr.grClient.UpdateStatus(grCopy)
		return err
	})
}

func getCRName(globalRole *v3.GlobalRole) string {
	if crName, ok := globalRole.Annotations[crNameAnnotation]; ok {
		return crName
	}
	return generateCRName(globalRole.Name)
}

func generateCRName(name string) string {
	return "cattle-globalrole-" + name
}

// validateNamespace checks if a namespace exists and is not terminating
// Returns (shouldSkip, error)
// - shouldSkip is true if the namespace is not found (warning logged, no error)
// - error is returned for other failures
func validateNamespace(nsCache wcorev1.NamespaceCache, ns, context string) (bool, error) {
	namespace, err := nsCache.Get(ns)
	if apierrors.IsNotFound(err) {
		logrus.Warnf("[%v] Namespace %s not found in %s. Continuing.", grController, ns, context)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("couldn't get namespace %s in %s: %w", ns, context, err)
	}
	if namespace == nil {
		return false, fmt.Errorf("couldn't get namespace %s in %s: namespace is nil", ns, context)
	}
	if namespace.Status.Phase == corev1.NamespaceTerminating {
		logrus.Warnf("[%v] Namespace %s is terminating in %s. Skipping role creation.", grController, ns, context)
		return true, nil
	}

	return false, nil
}

// ensureRoleLabels ensures a role has the correct owner label
func ensureRoleLabels(role *rbacv1.Role, ownerLabel string) bool {
	if role.Labels == nil {
		role.Labels = map[string]string{}
	}
	if role.Labels[grOwnerLabel] != ownerLabel {
		role.Labels[grOwnerLabel] = ownerLabel
		return true
	}
	return false
}

// areRolesEqual compares the Rules and Labels of two Roles and returns true if they are equal
func areRolesEqual(existingRole, desiredRole *rbacv1.Role) bool {
	return equality.Semantic.DeepEqual(desiredRole.Rules, existingRole.Rules) &&
		equality.Semantic.DeepEqual(desiredRole.Labels, existingRole.Labels)
}

// createOwnerLabelSelector creates a label selector for roles owned by a GlobalRole
func createOwnerLabelSelector(ownerLabel string) (labels.Selector, error) {
	r, err := labels.NewRequirement(grOwnerLabel, selection.Equals, []string{ownerLabel})
	if err != nil {
		return nil, fmt.Errorf("couldn't create label selector: %w", err)
	}
	return labels.NewSelector().Add(*r), nil
}

// deleteRolesByUID deletes roles that are not in the validUIDs set
func deleteRolesByUID(roles []*rbacv1.Role, validUIDs map[types.UID]struct{}, roleClient wrbacv1.RoleClient) error {
	var returnError error
	for _, role := range roles {
		if _, ok := validUIDs[role.UID]; !ok {
			returnError = errors.Join(returnError, rbac.DeleteNamespacedResource(role.Namespace, role.Name, roleClient))
		}
	}
	return returnError
}
