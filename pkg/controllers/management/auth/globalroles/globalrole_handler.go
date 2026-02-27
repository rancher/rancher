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
	wrangler "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

var (
	globalRoleLabel       = "authz.management.cattle.io/globalrole"
	crNameAnnotation      = "authz.management.cattle.io/cr-name"
	initialSyncAnnotation = "authz.management.cattle.io/initial-sync"
	clusterRoleKind       = "ClusterRole"
)

const (
	grOwnerLabel = "authz.management.cattle.io/gr-owner"
)

// Condition reason types
const (
	ClusterRoleExists                 = "ClusterRoleExists"
	NamespacedRuleRoleExists          = "NamespacedRuleRoleExists"
	InheritedNamespacedRuleRoleExists = "InheritedNamespacedRuleRoleExists"
	NamespaceNotAvailable             = "NamespaceNotAvailable"
	FailedToGetRole                   = "GetRoleFailed"
	FailedToCreateRole                = "CreateRoleFailed"
	FailedToUpdateRole                = "UpdateRoleFailed"
	FailedToGetNamespace              = "GetNamespaceFailed"
	FailedToCreateClusterRole         = "CreateClusterRoleFailed"
	FailedToUpdateClusterRole         = "UpdateClusterRoleFailed"
	FailedToGetCluster                = "GetClusterFailed"
	FailedToGetUserContext            = "GetUserContextFailed"
)

type fleetPermissionsRoleHandler interface {
	reconcileFleetWorkspacePermissions(globalRole *v3.GlobalRole) error
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
		grbCache:                management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		fleetPermissionsHandler: newFleetWorkspaceRoleHandler(management),
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
	grbCache                mgmtv3.GlobalRoleBindingCache
	fleetPermissionsHandler fleetPermissionsRoleHandler
}

func (gr *globalRoleLifecycle) Create(obj *v3.GlobalRole) (runtime.Object, error) {
	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status (status.Summary != "InProgress")
	// we don't need to perform a reconcile as nothing has changed.
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation && obj.Status.Summary != status.SummaryInProgress {
		return obj, nil
	}
	returnError := errors.Join(
		gr.setGRAsInProgress(obj), // set GR status to "in progress" while the underlying roles get added
		gr.reconcileGlobalRole(obj),
		gr.reconcileNamespacedRoles(obj),
		gr.reconcileInheritedNamespacedRoles(obj),
		gr.fleetPermissionsHandler.reconcileFleetWorkspacePermissions(obj),
		gr.setGRAsCompleted(obj),
	)
	return obj, returnError
}

func (gr *globalRoleLifecycle) Updated(obj *v3.GlobalRole) (runtime.Object, error) {
	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status (status.Summary != "InProgress")
	// we don't need to perform a reconcile as nothing has changed.
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation && obj.Status.Summary != status.SummaryInProgress {
		return obj, nil
	}

	returnError := errors.Join(
		gr.setGRAsInProgress(obj), // set GR status to "in progress" while the underlying roles get added
		gr.reconcileGlobalRole(obj),
		gr.reconcileNamespacedRoles(obj),
		gr.reconcileInheritedNamespacedRoles(obj),
		gr.fleetPermissionsHandler.reconcileFleetWorkspacePermissions(obj),
		gr.setGRAsCompleted(obj),
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
	err := errors.Join(
		gr.deleteInheritedNamespacedRoles(obj),
		gr.setGRAsTerminating(obj),
	)
	return nil, err
}

func (gr *globalRoleLifecycle) reconcileGlobalRole(globalRole *v3.GlobalRole) error {
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
				addCondition(globalRole, condition, FailedToUpdateClusterRole, crName, err)
				return fmt.Errorf("couldn't update ClusterRole %v: %w", clusterRole.Name, err)
			}
		}
		addCondition(globalRole, condition, ClusterRoleExists, crName, nil)
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
		addCondition(globalRole, condition, FailedToCreateClusterRole, crName, err)
		return err
	}
	// Add an annotation to the globalrole indicating the name we used for future updates
	if globalRole.Annotations == nil {
		globalRole.Annotations = map[string]string{}
	}
	globalRole.Annotations[crNameAnnotation] = crName
	addCondition(globalRole, condition, ClusterRoleExists, crName, nil)
	return nil
}

// reconcileNamespacedRoles ensures that Roles exist in each namespace of NamespacedRules
func (gr *globalRoleLifecycle) reconcileNamespacedRoles(globalRole *v3.GlobalRole) error {
	var returnError error
	globalRoleName := wrangler.SafeConcatName(globalRole.Name)

	// For collecting all the roles that should exist for the GlobalRole
	roleUIDs := map[types.UID]struct{}{}

	for ns, rules := range globalRole.NamespacedRules {
		roleName := wrangler.SafeConcatName(globalRole.Name, ns)
		condition := metav1.Condition{
			Type: NamespacedRuleRoleExists,
		}

		shouldSkip, err := validateNamespace(gr.nsCache, ns, "local cluster")
		if shouldSkip {
			addCondition(globalRole, condition, NamespaceNotAvailable, roleName, fmt.Errorf("namespace %s not available", ns))
			continue
		}
		if err != nil {
			returnError = errors.Join(returnError, err)
			addCondition(globalRole, condition, FailedToGetNamespace, roleName, err)
			continue
		}

		// Check if the role exists
		role, err := gr.rLister.Get(ns, roleName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				returnError = errors.Join(returnError, err)
				addCondition(globalRole, condition, FailedToGetRole, roleName, err)
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
			createdRole, err := gr.rClient.Create(newRole)
			if err == nil {
				roleUIDs[createdRole.UID] = struct{}{}
				addCondition(globalRole, condition, NamespacedRuleRoleExists, roleName, nil)
				continue
			}

			if !apierrors.IsAlreadyExists(err) {
				returnError = errors.Join(returnError, err)
				addCondition(globalRole, condition, FailedToCreateRole, roleName, err)
				continue
			}

			// In the case that the role already exists, we get it and check that the rules are correct
			role, err = gr.rLister.Get(ns, roleName)
			if err != nil {
				returnError = errors.Join(returnError, err)
				addCondition(globalRole, condition, FailedToGetRole, roleName, err)
				continue
			}
		}
		if role != nil {
			roleUIDs[role.GetUID()] = struct{}{}

			// Check that the rules for the existing role are correct and that it has the right Owner Label
			if !needsRoleUpdate(role, rules, globalRoleName) {
				addCondition(globalRole, condition, NamespacedRuleRoleExists, roleName, nil)
				continue
			}

			newRole := role.DeepCopy()
			newRole.Rules = rules
			ensureRoleLabels(newRole, globalRoleName)

			_, err := gr.rClient.Update(newRole)
			if err != nil {
				returnError = errors.Join(returnError, err)
				addCondition(globalRole, condition, FailedToUpdateRole, roleName, err)
				continue
			}
			addCondition(globalRole, condition, NamespacedRuleRoleExists, roleName, nil)
		}
	}

	// get all the roles claiming to be owned by this GR and remove any that shouldn't exist
	selector, err := createOwnerLabelSelector(globalRoleName)
	if err != nil {
		return errors.Join(returnError, err)
	}

	roles, err := gr.rLister.List("", selector)
	if err != nil {
		return errors.Join(returnError, fmt.Errorf("couldn't list roles with label %s: %s: %w", grOwnerLabel, globalRoleName, err))
	}

	// After creating/updating all Roles, if the number of RBs with the grOwnerLabel is the same as
	// as the number of created/updated Roles, we know there are no invalid Roles to purge
	if len(roleUIDs) != len(roles) {
		err = deleteRolesByUID(roles, roleUIDs, gr.rClient)
		if err != nil {
			returnError = errors.Join(returnError, err)
		}
	}
	return returnError
}

// reconcileInheritedNamespacedRoles ensures that Roles exist in each namespace of InheritedNamespacedRules for all downstream clusters
func (gr *globalRoleLifecycle) reconcileInheritedNamespacedRoles(globalRole *v3.GlobalRole) error {
	// If there are no InheritedNamespacedRules, nothing to do
	if len(globalRole.InheritedNamespacedRules) == 0 {
		return nil
	}

	var returnError error
	globalRoleName := wrangler.SafeConcatName(globalRole.Name)
	hasError := false

	// Get all clusters
	clusters, err := gr.clusters.List(metav1.ListOptions{})
	if err != nil {
		condition := metav1.Condition{
			Type: InheritedNamespacedRuleRoleExists,
		}
		addCondition(globalRole, condition, FailedToGetCluster, "InheritedNamespacedRules", err)
		return fmt.Errorf("couldn't list clusters: %w", err)
	}

	// Track all role UIDs that should exist across all clusters
	roleUIDs := map[types.UID]struct{}{}

	// Iterate through all clusters except local
	for _, cluster := range clusters.Items {
		if cluster.Name == "local" {
			continue
		}

		clusterErr := gr.reconcileInheritedNamespacedRolesForCluster(&cluster, globalRole, globalRoleName, roleUIDs)
		if clusterErr != nil {
			returnError = errors.Join(returnError, clusterErr)
			hasError = true
		}
	}

	// Add a single condition for all InheritedNamespacedRules
	condition := metav1.Condition{
		Type: InheritedNamespacedRuleRoleExists,
	}
	if hasError {
		addCondition(globalRole, condition, FailedToCreateRole, "InheritedNamespacedRules", returnError)
	} else {
		addCondition(globalRole, condition, InheritedNamespacedRuleRoleExists, "InheritedNamespacedRules", nil)
	}

	return returnError
}

// reconcileInheritedNamespacedRolesForCluster reconciles roles for a single downstream cluster
func (gr *globalRoleLifecycle) reconcileInheritedNamespacedRolesForCluster(cluster *v3.Cluster, globalRole *v3.GlobalRole, globalRoleName string, roleUIDs map[types.UID]struct{}) error {
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

	// Iterate through all namespaces in InheritedNamespacedRules
	for ns, rules := range globalRole.InheritedNamespacedRules {
		nsErr := gr.reconcileInheritedRoleInNamespace(cluster.Name, ns, rules, globalRole.Name, globalRoleName, roleClient, roleCache, namespaceCache, roleUIDs)
		if nsErr != nil {
			returnError = errors.Join(returnError, nsErr)
		}
	}

	// Purge invalid roles in this cluster
	purgeErr := gr.purgeInvalidInheritedRolesInCluster(cluster.Name, globalRoleName, roleCache, roleClient, roleUIDs)
	if purgeErr != nil {
		returnError = errors.Join(returnError, purgeErr)
	}

	return returnError
}

// reconcileInheritedRoleInNamespace reconciles a single role in a specific namespace of a downstream cluster
func (gr *globalRoleLifecycle) reconcileInheritedRoleInNamespace(clusterName, ns string, rules []rbacv1.PolicyRule, globalRoleName, safeGlobalRoleName string, roleClient wrbacv1.RoleClient, roleCache wrbacv1.RoleCache, namespaceCache wcorev1.NamespaceCache, roleUIDs map[types.UID]struct{}) error {
	roleName := wrangler.SafeConcatName(globalRoleName, ns)

	// Check if the namespace exists in this cluster
	shouldSkip, err := validateNamespace(namespaceCache, ns, fmt.Sprintf("cluster %s", clusterName))
	if shouldSkip {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s in cluster %s: %w", err.Error(), clusterName, err)
	}

	// Check if the role exists
	role, err := roleCache.Get(ns, roleName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("couldn't get role %s in namespace %s in cluster %s: %w", roleName, ns, clusterName, err)
		}

		// Create the role
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
		createdRole, err := roleClient.Create(newRole)
		if err == nil {
			roleUIDs[createdRole.UID] = struct{}{}
			return nil
		}

		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create role %s in namespace %s in cluster %s: %w", roleName, ns, clusterName, err)
		}

		// If it already exists, get it and check that the rules are correct
		role, err = roleCache.Get(ns, roleName)
		if err != nil {
			return fmt.Errorf("couldn't get existing role %s in namespace %s in cluster %s: %w", roleName, ns, clusterName, err)
		}
	}

	if role != nil {
		roleUIDs[role.GetUID()] = struct{}{}

		// Check that the rules for the existing role are correct and that it has the right Owner Label
		if !needsRoleUpdate(role, rules, safeGlobalRoleName) {
			return nil
		}

		newRole := role.DeepCopy()
		newRole.Rules = rules
		ensureRoleLabels(newRole, safeGlobalRoleName)

		_, err := roleClient.Update(newRole)
		if err != nil {
			return fmt.Errorf("couldn't update role %s in namespace %s in cluster %s: %w", roleName, ns, clusterName, err)
		}
	}

	return nil
}

// purgeInvalidInheritedRolesInCluster removes roles in a cluster that are no longer needed
func (gr *globalRoleLifecycle) purgeInvalidInheritedRolesInCluster(clusterName, globalRoleName string, roleCache wrbacv1.RoleCache, roleClient wrbacv1.RoleClient, validRoleUIDs map[types.UID]struct{}) error {
	selector, err := createOwnerLabelSelector(globalRoleName)
	if err != nil {
		return fmt.Errorf("cluster %s: %w", clusterName, err)
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
	globalRoleName := wrangler.SafeConcatName(globalRole.Name)

	// Get all clusters
	clusters, err := gr.clusters.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't list clusters: %w", err)
	}

	// Iterate through all clusters except local
	for _, cluster := range clusters.Items {
		if cluster.Name == "local" {
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

		// Get all roles owned by this GlobalRole
		selector, err := createOwnerLabelSelector(globalRoleName)
		if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("cluster %s: %w", cluster.Name, err))
			continue
		}

		roles, err := roleCache.List("", selector)
		if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("couldn't list roles in cluster %s: %w", cluster.Name, err))
			continue
		}

		// Delete all roles owned by this GlobalRole
		deleteErr := deleteRolesByUID(roles, map[types.UID]struct{}{}, roleClient)
		if deleteErr != nil {
			returnError = errors.Join(returnError, deleteErr)
		}
	}

	return returnError
}

func (gr *globalRoleLifecycle) setGRAsInProgress(globalRole *v3.GlobalRole) error {
	globalRole.Status.Conditions = []metav1.Condition{}
	globalRole.Status.Summary = status.SummaryInProgress
	globalRole.Status.LastUpdate = time.Now().UTC().Format(time.RFC3339)
	updatedGR, err := gr.grClient.UpdateStatus(globalRole)
	// For future updates, we want the latest version of our GlobalRole
	*globalRole = *updatedGR
	return err
}

func (gr *globalRoleLifecycle) setGRAsCompleted(globalRole *v3.GlobalRole) error {
	globalRole.Status.Summary = status.SummaryCompleted
	for _, c := range globalRole.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			globalRole.Status.Summary = status.SummaryError
			break
		}
	}
	globalRole.Status.LastUpdate = time.Now().UTC().Format(time.RFC3339)
	globalRole.Status.ObservedGeneration = globalRole.ObjectMeta.Generation
	_, err := gr.grClient.UpdateStatus(globalRole)
	return err
}

func (gr *globalRoleLifecycle) setGRAsTerminating(globalRole *v3.GlobalRole) error {
	globalRole.Status.Conditions = []metav1.Condition{}
	globalRole.Status.Summary = status.SummaryTerminating
	globalRole.Status.LastUpdate = time.Now().UTC().Format(time.RFC3339)
	_, err := gr.grClient.UpdateStatus(globalRole)
	return err
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

func addCondition(globalRole *v3.GlobalRole, condition metav1.Condition, reason, name string, err error) {
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Message = fmt.Sprintf("%s not created: %v", name, err)
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Message = fmt.Sprintf("%s created", name)
	}
	condition.Reason = reason
	condition.LastTransitionTime = metav1.Time{Time: time.Now()}
	globalRole.Status.Conditions = append(globalRole.Status.Conditions, condition)
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
	if err != nil || namespace == nil {
		return false, fmt.Errorf("couldn't get namespace %s in %s: %w", ns, context, err)
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

// needsRoleUpdate checks if a role needs updating based on rules and labels
func needsRoleUpdate(role *rbacv1.Role, rules []rbacv1.PolicyRule, ownerLabel string) bool {
	if !reflect.DeepEqual(role.Rules, rules) {
		return true
	}
	if role.Labels == nil || role.Labels[grOwnerLabel] != ownerLabel {
		return true
	}
	return false
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
