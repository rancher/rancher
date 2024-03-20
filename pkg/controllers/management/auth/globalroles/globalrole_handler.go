package globalroles

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	mgmtconv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	wrangler "github.com/rancher/wrangler/v2/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
)

var (
	globalRoleLabel       = map[string]string{"authz.management.cattle.io/globalrole": "true"}
	crNameAnnotation      = "authz.management.cattle.io/cr-name"
	initialSyncAnnotation = "authz.management.cattle.io/initial-sync"
	clusterRoleKind       = "ClusterRole"
)

const (
	grOwnerLabel = "authz.management.cattle.io/gr-owner"
)

const (
	SummaryInProgress = "InProgress"
	SummaryCompleted  = "Completed"
	SummaryError      = "Error"
)

// Condition reason types
const (
	ClusterRoleExists         = "ClusterRoleExists"
	NamespacedRuleRoleExists  = "NamespacedRuleRoleExists"
	CatalogRoleExists         = "CatalogRoleExists"
	NamespaceNotFound         = "NamespaceNotFound"
	NamespaceTerminating      = "NamespaceTerminating"
	FailedToGetRole           = "GetRoleFailed"
	FailedToCreateRole        = "CreateRoleFailed"
	FailedToUpdateRole        = "UpdateRoleFailed"
	FailedToGetNamespace      = "GetNamespaceFailed"
	FailedToCreateClusterRole = "CreateClusterRoleFailed"
	FailedToUpdateClusterRole = "UpdateClusterRoleFailed"
)

func newGlobalRoleLifecycle(management *config.ManagementContext) *globalRoleLifecycle {
	return &globalRoleLifecycle{
		crLister: management.RBAC.ClusterRoles("").Controller().Lister(),
		crClient: management.RBAC.ClusterRoles(""),
		nsCache:  management.Wrangler.Core.Namespace().Cache(),
		rLister:  management.RBAC.Roles("").Controller().Lister(),
		rClient:  management.RBAC.Roles(""),
		grClient: management.Wrangler.Mgmt.GlobalRole(),
	}
}

type globalRoleLifecycle struct {
	crLister rbacv1.ClusterRoleLister
	crClient rbacv1.ClusterRoleInterface
	nsCache  wcorev1.NamespaceCache
	rLister  rbacv1.RoleLister
	rClient  rbacv1.RoleInterface
	grClient mgmtconv3.GlobalRoleClient
}

func (gr *globalRoleLifecycle) Create(obj *v3.GlobalRole) (runtime.Object, error) {
	var returnError error

	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status (status.Summary != "InProgress")
	// we don't need to perform a reconcile as nothing has changed.
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation && obj.Status.Summary != SummaryInProgress {
		return obj, nil
	}
	// set GR status to "in progress" while the underlying roles get added
	err := gr.setGRAsInProgress(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileGlobalRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileCatalogRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileNamespacedRoles(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.setGRAsCompleted(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	return obj, returnError
}

func (gr *globalRoleLifecycle) Updated(obj *v3.GlobalRole) (runtime.Object, error) {
	var returnError error

	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status (status.Summary != "InProgress")
	// we don't need to perform a reconcile as nothing has changed.
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation && obj.Status.Summary != SummaryInProgress {
		return obj, nil
	}
	// set GR status to "in progress" while the underlying roles get added
	err := gr.setGRAsInProgress(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileGlobalRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileCatalogRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileNamespacedRoles(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.setGRAsCompleted(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	return nil, returnError
}

func (gr *globalRoleLifecycle) Remove(obj *v3.GlobalRole) (runtime.Object, error) {
	// Don't need to delete the created ClusterRole or Roles because owner reference will take care of them
	return nil, nil
}

func (gr *globalRoleLifecycle) reconcileGlobalRole(globalRole *v3.GlobalRole) error {
	crName := getCRName(globalRole)
	condition := metav1.Condition{
		Type: ClusterRoleExists,
	}

	clusterRole, _ := gr.crLister.Get("", crName)
	if clusterRole != nil {
		if !reflect.DeepEqual(globalRole.Rules, clusterRole.Rules) {
			clusterRole.Rules = globalRole.Rules
			logrus.Infof("[%v] Updating clusterRole %v. GlobalRole rules have changed. Have: %+v. Want: %+v", grController, clusterRole.Name, clusterRole.Rules, globalRole.Rules)
			if _, err := gr.crClient.Update(clusterRole); err != nil {
				addCondition(globalRole, condition, FailedToUpdateClusterRole, crName, err)
				return errors.Wrapf(err, "couldn't update ClusterRole %v", clusterRole.Name)
			}
		}
		addCondition(globalRole, condition, ClusterRoleExists, crName, nil)
		return nil
	}

	logrus.Infof("[%v] Creating clusterRole %v for corresponding GlobalRole", grController, crName)
	_, err := gr.crClient.Create(&v1.ClusterRole{
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
			Labels: globalRoleLabel,
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

func (gr *globalRoleLifecycle) reconcileCatalogRole(globalRole *v3.GlobalRole) error {
	// rules which give template/template version access need to have a specific namespaced role created, since the
	// backend resources that they grant access to are namespaced resources
	var catalogRules []v1.PolicyRule
	for _, rule := range globalRole.Rules {
		ruleGivesTemplateAccess := rbac.RuleGivesResourceAccess(rule, TemplateResourceRule)
		ruleGivesTemplateVersionAccess := rbac.RuleGivesResourceAccess(rule, TemplateVersionResourceRule)
		if !(ruleGivesTemplateAccess || ruleGivesTemplateVersionAccess) {
			// if rule doesn't give access to templates or template versions, move on without evaluating further
			continue
		}
		ruleCopy := rule.DeepCopy()
		ruleCopy.APIGroups = []string{mgmt.GroupName}
		ruleCopy.Resources = []string{}
		// NonResource URLS are only used for ClusterRoles - these roles are namespaced, so no need to include
		ruleCopy.NonResourceURLs = []string{}
		if ruleGivesTemplateAccess {
			ruleCopy.Resources = append(ruleCopy.Resources, catalogTemplateResourceRule)
		}
		if ruleGivesTemplateVersionAccess {
			ruleCopy.Resources = append(ruleCopy.Resources, catalogTemplateVersionResourceRule)
		}
		catalogRules = append(catalogRules, *ruleCopy)
	}
	if len(catalogRules) == 0 {
		return nil
	}
	// if this GR gives access to templates/template versions, create a role in cattle-global-data for access
	roleName := globalRole.Name + "-" + GlobalCatalogRole
	condition := metav1.Condition{
		Type: CatalogRoleExists,
	}
	role, err := gr.rLister.Get(namespace.GlobalNamespace, roleName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			addCondition(globalRole, condition, FailedToGetRole, roleName, err)
			return err
		}
		role = &v1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: namespace.GlobalNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: globalRole.APIVersion,
						Kind:       globalRole.Kind,
						Name:       globalRole.Name,
						UID:        globalRole.UID,
					},
				},
			},
			Rules: catalogRules,
		}
		_, err = gr.rClient.Create(role)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			addCondition(globalRole, condition, FailedToCreateRole, roleName, err)
			return err
		}
	} else {
		// if we found the role, make sure that the rules are up-to-date and give the same access as their grs
		updateRule := false
		for _, rule := range catalogRules {
			ruleFound := false
			for _, existingRule := range globalRole.Rules {
				if reflect.DeepEqual(rule, existingRule) {
					ruleFound = true
					break
				}
			}
			if !ruleFound {
				// if we need to update any individual rule, just replace them all
				updateRule = true
				break
			}
		}
		if updateRule {
			newRole := role.DeepCopy()
			newRole.Rules = catalogRules
			_, err := gr.rClient.Update(newRole)
			if err != nil {
				addCondition(globalRole, condition, FailedToUpdateRole, roleName, err)
				return err
			}
		}
	}
	addCondition(globalRole, condition, CatalogRoleExists, roleName, nil)
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

		namespace, err := gr.nsCache.Get(ns)
		if apierrors.IsNotFound(err) || namespace == nil {
			// When a namespace is not found, don't re-enqueue GlobalRole
			logrus.Warnf("[%v] Namespace %s not found. Not re-enqueueing GlobalRole %s", grController, ns, globalRole.Name)
			addCondition(globalRole, condition, NamespaceNotFound, roleName, fmt.Errorf("namespace %s not found", ns))
			continue
		} else if err != nil {
			returnError = multierror.Append(returnError, errors.Wrapf(err, "couldn't get namespace %s", ns))
			addCondition(globalRole, condition, FailedToGetNamespace, roleName, err)
			continue
		}

		// Check if the role exists
		role, err := gr.rLister.Get(ns, roleName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				returnError = multierror.Append(returnError, err)
				addCondition(globalRole, condition, FailedToGetRole, roleName, err)
				continue
			}

			// If the namespace is terminating, don't create a Role
			if namespace.Status.Phase == corev1.NamespaceTerminating {
				logrus.Warnf("[%v] Namespace %s is terminating. Not creating role %s for %s", grController, ns, roleName, globalRole.Name)
				addCondition(globalRole, condition, NamespaceTerminating, roleName, fmt.Errorf("namespace %s is terminating", ns))
				continue
			}

			newRole := &v1.Role{
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
				returnError = multierror.Append(returnError, err)
				addCondition(globalRole, condition, FailedToCreateRole, roleName, err)
				continue
			}

			// In the case that the role already exists, we get it and check that the rules are correct
			role, err = gr.rLister.Get(ns, roleName)
			if err != nil {
				returnError = multierror.Append(returnError, err)
				addCondition(globalRole, condition, FailedToGetRole, roleName, err)
				continue
			}
		}
		if role != nil {
			roleUIDs[role.GetUID()] = struct{}{}

			// Check that the rules for the existing role are correct and that it has the right Owner Label
			if reflect.DeepEqual(role.Rules, rules) && role.Labels != nil && role.Labels[grOwnerLabel] == globalRoleName {
				addCondition(globalRole, condition, NamespacedRuleRoleExists, roleName, nil)
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
				returnError = multierror.Append(returnError, err)
				addCondition(globalRole, condition, FailedToUpdateRole, roleName, err)
				continue
			}
			addCondition(globalRole, condition, NamespacedRuleRoleExists, roleName, nil)
		}
	}

	// get all the roles claiming to be owned by this GR and remove any that shouldn't exist
	r, err := labels.NewRequirement(grOwnerLabel, selection.Equals, []string{globalRoleName})
	if err != nil {
		return multierror.Append(returnError, errors.Wrapf(err, "couldn't create label: %s", grOwnerLabel))
	}

	roles, err := gr.rLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		return multierror.Append(returnError, errors.Wrapf(err, "couldn't list roles with label %s : %s", grOwnerLabel, globalRoleName))
	}

	// After creating/updating all Roles, if the number of RBs with the grOwnerLabel is the same as
	// as the number of created/updated Roles, we know there are no invalid Roles to purge
	if len(roleUIDs) != len(roles) {
		err = gr.purgeInvalidNamespacedRoles(roles, roleUIDs)
		if err != nil {
			returnError = multierror.Append(returnError, err)
		}
	}
	return returnError
}

// purgeInvalidNamespacedRoles removes any roles that aren't in the slice of UIDS that we created/updated in reconcileNamespacedRoles
func (gr *globalRoleLifecycle) purgeInvalidNamespacedRoles(roles []*v1.Role, uids map[types.UID]struct{}) error {
	var returnError error
	for _, r := range roles {
		if _, ok := uids[r.UID]; !ok {
			err := gr.rClient.DeleteNamespaced(r.Namespace, r.Name, &metav1.DeleteOptions{})
			if err != nil {
				returnError = multierror.Append(returnError, errors.Wrapf(err, "couldn't delete role %s", r.Name))
			}
		}
	}
	return returnError
}

func (gr *globalRoleLifecycle) setGRAsInProgress(globalRole *v3.GlobalRole) error {
	globalRole.Status.Conditions = []metav1.Condition{}
	globalRole.Status.Summary = SummaryInProgress
	globalRole.Status.LastUpdate = time.Now().String()
	updatedGR, err := gr.grClient.UpdateStatus(globalRole)
	// For future updates, we want the latest version of our GlobalRole
	*globalRole = *updatedGR
	return err
}

func (gr *globalRoleLifecycle) setGRAsCompleted(globalRole *v3.GlobalRole) error {
	globalRole.Status.Summary = SummaryCompleted
	for _, c := range globalRole.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			globalRole.Status.Summary = SummaryError
			break
		}
	}
	globalRole.Status.LastUpdate = time.Now().String()
	globalRole.Status.ObservedGeneration = globalRole.ObjectMeta.Generation
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
