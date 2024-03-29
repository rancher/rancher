package rbac

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

const owner = "owner-user"

// globalResourceRulesNeededInProjects is the set of PolicyRules that need to be present on *-promoted ClusterRoles
// Binding a user to a *-promoted ClusterRole with these PolicyRules results in that user being granted access to these "global" resources.
// The PolicyRule for each resource is the base policy rule, verbs are added dynamically and the key is used for the Resources value of each rule
var globalResourceRulesNeededInProjects = map[string]rbacv1.PolicyRule{
	"navlinks": rbacv1.PolicyRule{
		APIGroups: []string{"ui.cattle.io"},
	},
	"nodes": rbacv1.PolicyRule{
		APIGroups: []string{""},
	},
	"persistentvolumes": rbacv1.PolicyRule{
		APIGroups: []string{"", "core"},
	},
	"storageclasses": rbacv1.PolicyRule{
		APIGroups: []string{"storage.k8s.io"},
	},
	"apiservices": rbacv1.PolicyRule{
		APIGroups: []string{"apiregistration.k8s.io"},
	},
	"clusterrepos": rbacv1.PolicyRule{
		APIGroups: []string{"catalog.cattle.io"},
	},
	"clusters": rbacv1.PolicyRule{
		APIGroups: []string{"management.cattle.io"},
		// since *-promoted roles may be applied in all clusters, the resource name needs to be "local"
		// performing 'kubectl get clusters.management.cattle.io local' needs to work for the user within the context of the cluster they are promoted in,
		// otherwise certain functionality that relies on this global permission will not work correctly, e.g. kubectl shell
		// this will not grant the user permissions on the management cluster unless they are added as a project member/owner/read-only within that cluster
		ResourceNames: []string{"local"},
	},
}

func newPRTBLifecycle(m *manager) *prtbLifecycle {
	return &prtbLifecycle{m: m}
}

type prtbLifecycle struct {
	m *manager
}

func (p *prtbLifecycle) Create(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	err := p.syncPRTB(obj)
	return obj, err
}

func (p *prtbLifecycle) Updated(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	if err := p.reconcilePRTBUserClusterLabels(obj); err != nil {
		return obj, err
	}
	err := p.syncPRTB(obj)
	return obj, err
}

func (p *prtbLifecycle) Remove(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	err := p.ensurePRTBDelete(obj)
	return obj, err
}

func (p *prtbLifecycle) syncPRTB(binding *v3.ProjectRoleTemplateBinding) error {
	if binding.RoleTemplateName == "" {
		logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		return nil
	}

	rt, err := p.m.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return fmt.Errorf("couldn't get role template %v: %w", binding.RoleTemplateName, err)
	}

	// Get namespaces belonging to project
	namespaces, err := p.m.nsIndexer.ByIndex(nsByProjectIndex, binding.ProjectName)
	if err != nil {
		return fmt.Errorf("couldn't list namespaces with project ID %v: %w", binding.ProjectName, err)
	}
	roles := map[string]*v3.RoleTemplate{}
	if err := p.m.gatherRoles(rt, roles, 0); err != nil {
		return err
	}

	if err := p.m.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		if !ns.DeletionTimestamp.IsZero() {
			continue
		}
		if err := p.m.ensureProjectRoleBindings(ns.Name, roles, binding); err != nil {
			return fmt.Errorf("couldn't ensure binding %v in %v: %w", binding.Name, ns.Name, err)
		}
	}

	if binding.UserName != "" {
		if err := p.m.ensureServiceAccountImpersonator(binding.UserName); err != nil {
			return fmt.Errorf("couldn't ensure service account impersonator: %w", err)
		}
	}

	return p.reconcileProjectAccessToGlobalResources(binding, roles)
}

func (p *prtbLifecycle) ensurePRTBDelete(binding *v3.ProjectRoleTemplateBinding) error {
	// Get namespaces belonging to project
	namespaces, err := p.m.nsIndexer.ByIndex(nsByProjectIndex, binding.ProjectName)
	if err != nil {
		return fmt.Errorf("couldn't list namespaces with project ID %v: %w", binding.ProjectName, err)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		bindingCli := p.m.workload.RBAC.RoleBindings(ns.Name)
		rbs, err := p.m.rbLister.List(ns.Name, set.AsSelector())
		if err != nil {
			return fmt.Errorf("couldn't list rolebindings with selector %s: %w", set.AsSelector(), err)
		}

		for _, rb := range rbs {
			if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("error deleting rolebinding %v: %w", rb.Name, err)
				}
			}
		}
	}

	if err := p.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		return errors.Wrap(err, "error deleting service account impersonator")
	}

	return p.reconcileProjectAccessToGlobalResourcesForDelete(binding)
}

func (p *prtbLifecycle) reconcileProjectAccessToGlobalResources(binding *v3.ProjectRoleTemplateBinding, rts map[string]*v3.RoleTemplate) error {
	roles, err := p.m.ensureGlobalResourcesRolesForPRTB(parseProjectName(binding.ProjectName), rts)
	if err != nil {
		return err
	}
	_, err = p.m.reconcileProjectAccessToGlobalResources(binding, roles)
	if err != nil {
		return err
	}
	return nil
}

func (p *prtbLifecycle) reconcileProjectAccessToGlobalResourcesForDelete(binding *v3.ProjectRoleTemplateBinding) error {
	bindingCli := p.m.workload.RBAC.ClusterRoleBindings("")
	rtbNsAndName := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	set := labels.Set(map[string]string{rtbNsAndName: owner})
	crbs, err := p.m.crbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}

	for _, crb := range crbs {
		crb = crb.DeepCopy()
		for k, v := range crb.Labels {
			if k == rtbNsAndName && v == owner {
				delete(crb.Labels, k)
			}
		}

		eligibleForDeletion, err := p.m.noRemainingOwnerLabels(crb)
		if err != nil {
			return err
		}

		if eligibleForDeletion {
			if err := bindingCli.Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
		} else {
			if _, err := bindingCli.Update(crb); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *manager) noRemainingOwnerLabels(crb *rbacv1.ClusterRoleBinding) (bool, error) {
	for k, v := range crb.Labels {
		if v == owner {
			if exists, err := m.ownerExists(k); exists || err != nil {
				return false, err
			}
			if exists, err := m.ownerExistsByNsName(k); exists || err != nil {
				return false, err
			}
		}

		if k == rtbOwnerLabelLegacy {
			if exists, err := m.ownerExists(v); exists || err != nil {
				return false, err
			}
		}
		if k == rtbOwnerLabel {
			if exists, err := m.ownerExistsByNsName(v); exists || err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func (m *manager) ownerExists(uid interface{}) (bool, error) {
	prtbs, err := m.prtbIndexer.ByIndex(prtbByUIDIndex, convert.ToString(uid))
	return len(prtbs) > 0, err
}

func (m *manager) ownerExistsByNsName(nsAndName interface{}) (bool, error) {
	prtbs, err := m.prtbIndexer.ByIndex(prtbByNsAndNameIndex, convert.ToString(nsAndName))
	return len(prtbs) > 0, err
}

// If the roleTemplate has rules granting access to non-namespaced (global) resource, return the verbs for those rules
func (m *manager) checkForGlobalResourceRules(role *v3.RoleTemplate, resource string, baseRule rbacv1.PolicyRule) (sets.Set[string], error) {
	var rules []rbacv1.PolicyRule
	if role.External {
		externalRole, err := m.crLister.Get("", role.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			// dont error if it doesnt exist
			return nil, err
		}
		if externalRole != nil {
			rules = externalRole.Rules
		}
	} else {
		rules = role.Rules
	}

	verbs := sets.New[string]()
	for _, rule := range rules {
		// given the global resource, we check if the passed in RoleTemplate has a corresponding rule, if it does, we add the verbs specified in the rule to the map of verbs that is returned
		// NOTE: ResourceNames are checked since some global resources are scoped to specific resources, e.g. management.cattle.io/v3.Clusters are scoped to just the "local" cluster resource
		if (slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, "*")) && reflect.DeepEqual(rule.ResourceNames, baseRule.ResourceNames) {
			if checkGroup(resource, rule) {
				verbs.Insert(rule.Verbs...)
			}
		}
	}

	return verbs, nil
}

// reconcileRoleForProjectAccessToGlobalResource ensure the clusterRole used to grant access of global resources
// to users/groups in projects has appropriate rules for the given resource and verbs.
// It returns the created or updated ClusterRole name, or blank "" if none were created or updated.
// The roleName is used to find and create/update the relevant '<roleName>-promoted' ClusterRole.
func (m *manager) reconcileRoleForProjectAccessToGlobalResource(resource string, roleName string, newVerbs sets.Set[string], baseRule rbacv1.PolicyRule) (string, error) {
	if roleName == "" {
		return "", errors.New("cannot reconcile Role: missing roleName")
	}
	roleName = roleName + "-promoted"

	role, err := m.crLister.Get("", roleName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("get cluster role %s failed: %w", roleName, err)
		}

		// try to create the role if not found

		// if newVerbs are empty we can skip the creation and return a blank role name
		// to let the caller knows that this was a no-op
		if len(newVerbs) == 0 {
			return "", nil
		}

		logrus.Infof("Creating clusterRole %v for project access to global resource.", roleName)

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
			},
			Rules: []rbacv1.PolicyRule{buildRule(resource, newVerbs, baseRule)},
		}

		_, err := m.clusterRoles.Create(clusterRole)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return "", fmt.Errorf("couldn't create role %v: %w", roleName, err)
			}
			logrus.Infof("Trying to create an already existing clusterRole %v for project access to global resource.", roleName)
		}

		return roleName, nil
	}

	// role already exists -> updating / reconciling

	currentVerbs := sets.New[string]()
	currentResourceNames := sets.New[string]()
	for _, rule := range role.Rules {
		if slice.ContainsString(rule.Resources, resource) {
			currentVerbs.Insert(rule.Verbs...)
			currentResourceNames.Insert(rule.ResourceNames...)
		}
	}

	desiredResourceNames := sets.New(baseRule.ResourceNames...)

	// if the currentVerbs and the currentResourceNames matches the desired state we can return
	if currentVerbs.Equal(newVerbs) && currentResourceNames.Equal(desiredResourceNames) {
		return roleName, nil
	}

	// if the verbs or the resourceNames in the promoted clusterrole don't match what's desired then the role requires updating
	// desired verbs are passed in and the desired resourceNames come from the resource's base rule
	role = role.DeepCopy()

	added := false
	for i, rule := range role.Rules {
		if slice.ContainsString(rule.Resources, resource) {
			role.Rules[i] = buildRule(resource, newVerbs, baseRule)
			added = true
		}
	}
	if !added {
		role.Rules = append(role.Rules, buildRule(resource, newVerbs, baseRule))
	}

	// check if we need to delete some policy rules
	if len(newVerbs) == 0 {
		newRules := []rbacv1.PolicyRule{}
		for _, rule := range role.Rules {
			if !slice.ContainsString(rule.Resources, resource) {
				newRules = append(newRules, rule)
			}
		}
		role.Rules = newRules
	}

	logrus.Infof("Updating clusterRole %v for project access to global resource.", role.Name)
	_, err = m.clusterRoles.Update(role)
	if err != nil {
		return "", fmt.Errorf("couldn't update role %v: %w", role.Name, err)
	}
	return roleName, nil
}

// checkGroup returns true if the passed in PolicyRule has a group that matches the corresponding baseRule for the passed in global resource
func checkGroup(resource string, rule rbacv1.PolicyRule) bool {
	if slice.ContainsString(rule.APIGroups, "*") {
		return true
	}

	baseRule, ok := globalResourceRulesNeededInProjects[resource]
	if !ok {
		return false
	}

	for _, rg := range rule.APIGroups {
		if slice.ContainsString(baseRule.APIGroups, rg) {
			return true
		}
	}

	return false
}

func buildRule(resource string, verbs sets.Set[string], baseRule rbacv1.PolicyRule) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		Resources:     []string{resource},
		Verbs:         sets.List(verbs), // List returns a sorted array of the verbs
		APIGroups:     baseRule.APIGroups,
		ResourceNames: baseRule.ResourceNames,
	}
}

func (p *prtbLifecycle) reconcilePRTBUserClusterLabels(binding *v3.ProjectRoleTemplateBinding) error {
	/* Prior to 2.5, for every PRTB, following CRBs are created in the user clusters
		1. PRTB.UID is the label key for a CRB, PRTB.UID=owner-user
		2. PRTB.UID is the label value for RBs with authz.cluster.cattle.io/rtb-owner: PRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		return nil
	}

	var returnErr error
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(pkgrbac.GetRTBLabel(binding.ObjectMeta), selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set := labels.Set(map[string]string{string(binding.UID): owner})
	userCRBs, err := p.m.clusterRoleBindings.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		return err
	}
	bindingLabel := pkgrbac.GetRTBLabel(binding.ObjectMeta)

	for _, crb := range userCRBs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := p.m.clusterRoleBindings.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[bindingLabel] = owner
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := p.m.clusterRoleBindings.Update(crbToUpdate)
			return err
		})
		if retryErr != nil {
			returnErr = multierror.Append(returnErr, retryErr)
		}
	}

	reqUpdatedOwnerLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set = map[string]string{rtbOwnerLabelLegacy: string(binding.UID)}
	rbs, err := p.m.rbLister.List(v1.NamespaceAll, set.AsSelector().Add(*reqUpdatedLabel, *reqUpdatedOwnerLabel))
	if err != nil {
		return err
	}
	for _, rb := range rbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rbToUpdate, updateErr := p.m.roleBindings.GetNamespaced(rb.Namespace, rb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if rbToUpdate.Labels == nil {
				rbToUpdate.Labels = make(map[string]string)
			}
			rbToUpdate.Labels[rtbOwnerLabel] = bindingLabel
			rbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := p.m.roleBindings.Update(rbToUpdate)
			return err
		})
		if retryErr != nil {
			returnErr = multierror.Append(returnErr, retryErr)
		}
	}

	if returnErr != nil {
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		prtbToUpdate, updateErr := p.m.prtbs.GetNamespaced(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if prtbToUpdate.Labels == nil {
			prtbToUpdate.Labels = make(map[string]string)
		}
		prtbToUpdate.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := p.m.prtbs.Update(prtbToUpdate)
		return err
	})
	return retryErr
}

func parseProjectName(id string) string {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || len(parts[1]) == 0 {
		return ""
	}
	return parts[1]
}
