package rbac

import (
	"reflect"

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
	"k8s.io/client-go/util/retry"
)

const owner = "owner-user"

var globalResourcesNeededInProjects = map[string]map[string]bool{
	"persistentvolumes": {
		"":     true,
		"core": true,
	},
	"storageclasses": {
		"storage.k8s.io": true,
	},
	"apiservices": {
		"apiregistration.k8s.io": true,
	},
	"clusterrepos": {
		"catalog.cattle.io": true,
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
	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" && binding.ServiceAccount == "" {
		return nil
	}

	rt, err := p.m.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	// Get namespaces belonging to project
	namespaces, err := p.m.nsIndexer.ByIndex(nsByProjectIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
	}
	roles := map[string]*v3.RoleTemplate{}
	if err := p.m.gatherRoles(rt, roles); err != nil {
		return err
	}

	if err := p.m.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		if err := p.m.ensureProjectRoleBindings(ns.Name, roles, binding); err != nil {
			return errors.Wrapf(err, "couldn't ensure binding %v in %v", binding.Name, ns.Name)
		}
	}

	return p.reconcileProjectAccessToGlobalResources(binding, roles)
}

func (p *prtbLifecycle) ensurePRTBDelete(binding *v3.ProjectRoleTemplateBinding) error {
	// Get namespaces belonging to project
	namespaces, err := p.m.nsIndexer.ByIndex(nsByProjectIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		bindingCli := p.m.workload.RBAC.RoleBindings(ns.Name)
		rbs, err := p.m.rbLister.List(ns.Name, set.AsSelector())
		if err != nil {
			return errors.Wrapf(err, "couldn't list rolebindings with selector %s", set.AsSelector())
		}

		for _, rb := range rbs {
			if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return errors.Wrapf(err, "error deleting rolebinding %v", rb.Name)
				}
			}
		}
	}

	return p.reconcileProjectAccessToGlobalResourcesForDelete(binding)
}

func (p *prtbLifecycle) reconcileProjectAccessToGlobalResources(binding *v3.ProjectRoleTemplateBinding, rts map[string]*v3.RoleTemplate) error {
	_, err := p.m.reconcileProjectAccessToGlobalResources(binding, rts)
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
func (m *manager) checkForGlobalResourceRules(role *v3.RoleTemplate, resource string) (map[string]bool, error) {
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

	verbs := map[string]bool{}
	for _, rule := range rules {
		if (slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, "*")) && len(rule.ResourceNames) == 0 {
			if checkGroup(resource, rule) {
				for _, v := range rule.Verbs {
					verbs[v] = true
				}
			}
		}
	}

	return verbs, nil
}

// Ensure the clusterRole used to grant access of global resources to users/groups in projects has appropriate rulesfor the give resource and verbs
func (m *manager) reconcileRoleForProjectAccessToGlobalResource(resource string, rt *v3.RoleTemplate, newVerbs map[string]bool) (string, error) {
	clusterRoles := m.workload.RBAC.ClusterRoles("")
	roleName := rt.Name + "-promoted"
	if role, err := m.crLister.Get("", roleName); err == nil && role != nil {
		currentVerbs := map[string]bool{}
		for _, rule := range role.Rules {
			if slice.ContainsString(rule.Resources, resource) {
				for _, v := range rule.Verbs {
					currentVerbs[v] = true
				}
			}
		}

		if !reflect.DeepEqual(currentVerbs, newVerbs) {
			role = role.DeepCopy()
			added := false
			for i, rule := range role.Rules {
				if slice.ContainsString(rule.Resources, resource) {
					role.Rules[i] = buildRule(resource, newVerbs)
					added = true
				}
			}
			if !added {
				role.Rules = append(role.Rules, buildRule(resource, newVerbs))
			}
			logrus.Infof("Updating clusterRole %v for project access to global resource.", role.Name)
			_, err := clusterRoles.Update(role)
			return roleName, err
		}
		return roleName, nil
	}

	logrus.Infof("Creating clusterRole %v for project access to global resource.", roleName)
	rules := []rbacv1.PolicyRule{buildRule(resource, newVerbs)}
	_, err := clusterRoles.Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: rules,
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return roleName, errors.Wrapf(err, "couldn't create role %v", roleName)
	}

	return roleName, nil
}

func checkGroup(resource string, rule rbacv1.PolicyRule) bool {
	if slice.ContainsString(rule.APIGroups, "*") {
		return true
	}

	groups, ok := globalResourcesNeededInProjects[resource]
	if !ok {
		return false
	}

	for _, rg := range rule.APIGroups {
		if _, ok := groups[rg]; ok {
			return true
		}
	}
	return false
}

func buildRule(resource string, verbs map[string]bool) rbacv1.PolicyRule {
	var vs []string
	for v := range verbs {
		vs = append(vs, v)
	}
	return rbacv1.PolicyRule{
		Resources: []string{resource},
		Verbs:     vs,
		APIGroups: []string{"*"},
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
	reqNsAndNameLabel, err := labels.NewRequirement(binding.Namespace+"_"+binding.Name, selection.DoesNotExist, []string{})
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
		crtbToUpdate, updateErr := p.m.prtbs.GetNamespaced(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		binding.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := p.m.prtbs.Update(crtbToUpdate)
		return err
	})
	return retryErr
}
