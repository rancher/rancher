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
	"k8s.io/client-go/util/retry"
)

const (
	globalRoleLabel       = "authz.management.cattle.io/globalrole"
	crNameAnnotation      = "authz.management.cattle.io/cr-name"
	initialSyncAnnotation = "authz.management.cattle.io/initial-sync"
	clusterRoleKind       = "ClusterRole"
	grOwnerLabel          = "authz.management.cattle.io/gr-owner"
)

// Condition reason types
const (
	ClusterRoleExists         = "ClusterRoleExists"
	NamespacedRuleRoleExists  = "NamespacedRuleRoleExists"
	FailedToCreateClusterRole = "CreateClusterRoleFailed"
	FailedToUpdateClusterRole = "UpdateClusterRoleFailed"
	FailedToCreateLabel       = "CreateLabelFailed"
	FailedToListRoles         = "ListRolesFailed"
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

	// Don't need to delete the created ClusterRole or Roles because owner reference will take care of them
	return nil, nil
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

	logrus.Infof("[%v] Creating clusterRole %v for corresponding GlobalRole", grController, crName)
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
	globalRoleName := wrangler.SafeConcatName(globalRole.Name)

	// For collecting all the roles that should exist for the GlobalRole
	roleUIDs := map[types.UID]struct{}{}

	for ns, rules := range globalRole.NamespacedRules {
		roleName := wrangler.SafeConcatName(globalRole.Name, ns)

		namespace, err := gr.nsCache.Get(ns)
		if apierrors.IsNotFound(err) || namespace == nil {
			// When a namespace is not found, don't re-enqueue GlobalRole
			logrus.Warnf("[%v] Namespace %s not found. Not re-enqueueing GlobalRole %s", grController, ns, globalRole.Name)
			continue
		} else if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("couldn't get namespace %s: %w", ns, err))
			continue
		}

		// Check if the role exists
		role, err := gr.rLister.Get(ns, roleName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				returnError = errors.Join(returnError, err)
				continue
			}

			// If the namespace is terminating, don't create a Role
			if namespace.Status.Phase == corev1.NamespaceTerminating {
				logrus.Warnf("[%v] Namespace %s is terminating. Not creating role %s for %s", grController, ns, roleName, globalRole.Name)
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
				continue
			}

			if !apierrors.IsAlreadyExists(err) {
				returnError = errors.Join(returnError, err)
				continue
			}

			// In the case that the role already exists, we get it and check that the rules are correct
			role, err = gr.rLister.Get(ns, roleName)
			if err != nil {
				returnError = errors.Join(returnError, err)
				continue
			}
		}
		if role != nil {
			roleUIDs[role.GetUID()] = struct{}{}

			// Check that the rules for the existing role are correct and that it has the right Owner Label
			if reflect.DeepEqual(role.Rules, rules) && role.Labels != nil && role.Labels[grOwnerLabel] == globalRoleName {
				continue
			}

			newRole := role.DeepCopy()
			newRole.Rules = rules
			if newRole.Labels == nil {
				newRole.Labels = map[string]string{}
			}
			newRole.Labels[grOwnerLabel] = globalRoleName

			_, err := gr.rClient.Update(newRole)
			if err != nil {
				returnError = errors.Join(returnError, err)
				continue
			}
		}
	}

	// get all the roles claiming to be owned by this GR and remove any that shouldn't exist
	r, err := labels.NewRequirement(grOwnerLabel, selection.Equals, []string{globalRoleName})
	if err != nil {
		gr.status.AddCondition(localConditions, condition, FailedToCreateLabel, err)
		return errors.Join(returnError, fmt.Errorf("couldn't create label: %s: %w", grOwnerLabel, err))
	}

	roles, err := gr.rLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		gr.status.AddCondition(localConditions, condition, FailedToListRoles, err)
		return errors.Join(returnError, fmt.Errorf("couldn't list roles with label %s : %s: %w", grOwnerLabel, globalRoleName, err))
	}

	// After creating/updating all Roles, if the number of roles with the grOwnerLabel is the same as
	// the number of created/updated Roles, we know there are no invalid Roles to purge
	if len(roleUIDs) != len(roles) {
		err = gr.purgeInvalidNamespacedRoles(roles, roleUIDs)
		if err != nil {
			returnError = errors.Join(returnError, err)
		}
	}

	gr.status.AddCondition(localConditions, condition, NamespacedRuleRoleExists, returnError)
	return returnError
}

// purgeInvalidNamespacedRoles removes any roles that aren't in the slice of UIDS that we created/updated in reconcileNamespacedRoles
func (gr *globalRoleLifecycle) purgeInvalidNamespacedRoles(roles []*rbacv1.Role, uids map[types.UID]struct{}) error {
	var returnError error
	for _, r := range roles {
		if _, ok := uids[r.UID]; !ok {
			err := gr.rClient.Delete(r.Namespace, r.Name, &metav1.DeleteOptions{})
			if err != nil {
				returnError = errors.Join(returnError, fmt.Errorf("couldn't delete role %s: %w", r.Name, err))
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
