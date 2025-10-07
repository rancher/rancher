package rbac

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
)

const owner = "owner-user"

func newPRTBLifecycle(m *manager, management *config.ManagementContext, nsInformer cache.SharedIndexInformer) *prtbLifecycle {
	return &prtbLifecycle{
		m:          m,
		rtLister:   management.Management.RoleTemplates("").Controller().Lister(),
		nsIndexer:  nsInformer.GetIndexer(),
		rbLister:   m.workload.RBACw.RoleBinding().Cache(),
		rbClient:   m.workload.RBACw.RoleBinding(),
		crbLister:  m.workload.RBACw.ClusterRoleBinding().Cache(),
		crbClient:  m.workload.RBACw.ClusterRoleBinding(),
		crClient:   m.workload.RBACw.ClusterRole(),
		crLister:   m.workload.RBACw.ClusterRole().Cache(),
		prtbClient: management.Management.ProjectRoleTemplateBindings(""),
	}
}

type prtbLifecycle struct {
	m          managerInterface
	rtLister   v3.RoleTemplateLister
	nsIndexer  cache.Indexer
	rbLister   wrbacv1.RoleBindingCache
	rbClient   wrbacv1.RoleBindingClient
	crbLister  wrbacv1.ClusterRoleBindingCache
	crbClient  wrbacv1.ClusterRoleBindingClient
	crClient   wrbacv1.ClusterRoleClient
	crLister   wrbacv1.ClusterRoleCache
	prtbClient v3.ProjectRoleTemplateBindingInterface
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
		logrus.Warnf("ProjectRoleTemplateBinding %s has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		return nil
	}
	rt, err := p.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("ProjectRoleTemplateBinding %s sets a non-existing role template %s. Skipping.", binding.Name, binding.RoleTemplateName)
			return nil
		}
		return err
	}

	// Get namespaces belonging to project
	namespaces, err := p.nsIndexer.ByIndex(namespace.NsByProjectIndex, binding.ProjectName)
	if err != nil {
		return fmt.Errorf("couldn't list namespaces with project ID %v: %w", binding.ProjectName, err)
	}
	roles := map[string]*v3.RoleTemplate{}
	if err := p.m.gatherRoles(rt, roles, 0); err != nil {
		return err
	}

	if err := p.ensurePSAPermissions(binding, roles); err != nil {
		return fmt.Errorf("couldn't ensure psa permissions: %w", err)
	}

	if err := p.m.ensureRoles(roles); err != nil {
		return fmt.Errorf("couldn't ensure roles: %w", err)
	}

	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
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

// ensurePSAPermissions creates the necessary ClusterRole and ClusterRoleBinding
// to give the 'updatepsa' permission to the project.
func (p *prtbLifecycle) ensurePSAPermissions(binding *v3.ProjectRoleTemplateBinding, roles map[string]*v3.RoleTemplate) error {
	// extract project name from the format "cluster-name:project-name"
	_, projectName, found := strings.Cut(binding.ProjectName, ":")
	if !found {
		return fmt.Errorf("invalid project name format")
	}

	// create AttributesRecord to test if roles allow the updatepsa operation
	psaRec := authorizer.AttributesRecord{
		Verb:            "updatepsa",
		APIGroup:        management.GroupName,
		Resource:        v32.ProjectResourceName,
		Name:            binding.ProjectName,
		ResourceRequest: true,
	}

	// check if any of the RoleTemplates grant updatepsa permission
	hasUpdatePSAPermission := false
	for _, role := range roles {
		if rbac.RulesAllow(psaRec, role.Rules...) {
			hasUpdatePSAPermission = true
		}
	}

	// only create RBAC resources if user has updatepsa permission
	if !hasUpdatePSAPermission {
		return nil
	}

	// ensure ClusterRole exists with correct updatepsa rules
	psaCRWanted := addUpdatepsaClusterRole(projectName)
	psaCR, err := p.crLister.Get(psaCRWanted.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create ClusterRole if it doesn't exist
			psaCR, err = p.crClient.Create(psaCRWanted)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			return err
		}
	}

	// verify existing ClusterRole has the correct updatepsa rules
	if !reflect.DeepEqual(psaCR.Rules, psaCRWanted.Rules) {
		// if the CR have been modified, restore it
		psaCR = addUpdatepsaClusterRole(projectName)
		_, err = p.crClient.Update(psaCR)
		if err != nil {
			return err
		}
	}

	ref := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     psaCR.Name,
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		return err
	}

	psaCRBName := pkgrbac.NameForClusterRoleBinding(ref, subject)

	// create ClusterRoleBinding to bind the ClusterRole to the user/group
	psaCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: psaCRBName,
		},
		RoleRef:  ref,
		Subjects: []rbacv1.Subject{subject},
	}

	_, err = p.crbClient.Create(psaCRB)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// ensurePRTBDelete cleans up all RBAC resources (RoleBindings, ServiceAccount, etc)
// associated with a ProjectRoleTemplateBinding when it's deleted.
// This ensures proper cleanup and prevents orphaned permissions.
func (p *prtbLifecycle) ensurePRTBDelete(binding *v3.ProjectRoleTemplateBinding) error {
	// Get namespaces belonging to project
	namespaces, err := p.nsIndexer.ByIndex(namespace.NsByProjectIndex, binding.ProjectName)
	if err != nil {
		return fmt.Errorf("couldn't list namespaces with project ID %v: %w", binding.ProjectName, err)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		rbs, err := p.rbLister.List(ns.Name, set.AsSelector())
		if err != nil {
			return fmt.Errorf("couldn't list rolebindings with selector %s: %w", set.AsSelector(), err)
		}

		for _, rb := range rbs {
			if err := p.rbClient.Delete(ns.Name, rb.Name, &metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("error deleting rolebinding %v: %w", rb.Name, err)
				}
			}
		}
	}

	if err := p.ensurePSAPermissionsDelete(binding); err != nil {
		return fmt.Errorf("error deleting psa RBAC resources: %w", err)
	}

	if err := p.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		return fmt.Errorf("error deleting service account impersonator: %w", err)
	}

	return p.reconcileProjectAccessToGlobalResourcesForDelete(binding)
}

// ensurePSAPermissionsDelete removes the ClusterRole and ClusterRoleBinding
// that were created to grant 'updatepsa' permissions when a ProjectRoleTemplateBinding
// is deleted. This function only performs cleanup if the binding's RoleTemplate
// actually granted updatepsa permissions, ensuring proper cleanup of PSA-related
// RBAC resources without affecting unrelated bindings.
func (p *prtbLifecycle) ensurePSAPermissionsDelete(binding *v3.ProjectRoleTemplateBinding) error {
	psaRec := authorizer.AttributesRecord{
		Verb:            "updatepsa",
		APIGroup:        management.GroupName,
		Resource:        v32.ProjectResourceName,
		Name:            binding.ProjectName,
		ResourceRequest: true,
	}
	_, projectName, found := strings.Cut(binding.ProjectName, ":")
	if !found {
		return fmt.Errorf("invalid project name format")
	}
	prtbRoleTemplate, err := p.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return err
	}

	if rbac.RulesAllow(psaRec, prtbRoleTemplate.Rules...) {
		// delete the ClusterRoleBinding first
		// if ClusterRole is not used by other ClusterRoleBindings:
		// then delete the ClusterRole
		// skip the deletion otherwhise
		psaCRName := fmt.Sprintf("%s-namespaces-psa", projectName)
		ref := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     psaCRName,
		}
		subject, err := pkgrbac.BuildSubjectFromRTB(binding)
		if err != nil {
			return err
		}
		psaCRBName := pkgrbac.NameForClusterRoleBinding(ref, subject)
		err = p.crbClient.Delete(psaCRBName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting clusterrolebinding %v: %w", psaCRBName, err)
		}
		isUsed := p.isClusterRoleUsed(psaCRName)
		if !isUsed {
			err = p.crClient.Delete(psaCRName, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("error deleting clusterrole %v: %w", psaCRName, err)
			}
		}
	}

	return nil
}

// IsClusterRoleUsed checks if a ClusterRole is referenced by any ClusterRoleBinding
func (p *prtbLifecycle) isClusterRoleUsed(clusterRoleName string) bool {
	bindings, err := p.crbClient.List(metav1.ListOptions{})
	if err != nil {
		return false
	}

	for _, binding := range bindings.Items {
		if binding.RoleRef.Kind == "ClusterRole" && binding.RoleRef.Name == clusterRoleName {
			return true
		}
	}

	return false
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
	rtbNsAndName := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	set := labels.Set(map[string]string{rtbNsAndName: owner})
	crbs, err := p.crbLister.List(set.AsSelector())
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
			if err := p.crbClient.Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
		} else {
			if _, err := p.crbClient.Update(crb); err != nil {
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

// reconcileRoleForProjectAccessToGlobalResource ensure the clusterRole used to grant access of global resources
// to users/groups in projects has appropriate rules for the given resource and verbs.
// It returns the created or updated ClusterRole name, or blank "" if none were created or updated.
// The roleName is used to find and create/update the relevant '<roleName>-promoted' ClusterRole.
func (m *manager) reconcileRoleForProjectAccessToGlobalResource(roleName string, promotedRules []rbacv1.PolicyRule) (string, error) {
	if roleName == "" {
		return "", fmt.Errorf("cannot reconcile Role: missing roleName")
	}
	roleName = roleName + "-promoted"

	role, err := m.crLister.Get(roleName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("get cluster role %s failed: %w", roleName, err)
		}

		// try to create the role if not found

		// if promotedRules are empty we can skip the creation and return a blank role name
		// to let the caller knows that this was a no-op
		if len(promotedRules) == 0 {
			return "", nil
		}

		logrus.Infof("Creating clusterRole %v for project access to global resource.", roleName)

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
			},
			Rules: promotedRules,
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

	// If there shouldn't be a promoted clusterRole, remove it
	if len(promotedRules) == 0 {
		logrus.Infof("RoleTemplate has no promoted rules, removing clusterRole %s", role.Name)
		return "", m.clusterRoles.Delete(role.Name, &metav1.DeleteOptions{})

	}

	// if the rules are already correct, no need to update
	if reflect.DeepEqual(role.Rules, promotedRules) {
		return roleName, nil
	}

	role.Rules = promotedRules

	logrus.Infof("Updating clusterRole %s for project access to global resources", role.Name)
	if _, err := m.clusterRoles.Update(role); err != nil {
		return "", fmt.Errorf("couldn't update role %s: %w", role.Name, err)
	}
	return roleName, nil
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
	userCRBs, err := p.crbClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		return err
	}
	bindingLabel := pkgrbac.GetRTBLabel(binding.ObjectMeta)

	for _, crb := range userCRBs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := p.crbClient.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[bindingLabel] = owner
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := p.crbClient.Update(crbToUpdate)
			return err
		})
		returnErr = errors.Join(returnErr, retryErr)
	}

	reqUpdatedOwnerLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set = map[string]string{rtbOwnerLabelLegacy: string(binding.UID)}
	rbs, err := p.rbLister.List(corev1.NamespaceAll, set.AsSelector().Add(*reqUpdatedLabel, *reqUpdatedOwnerLabel))
	if err != nil {
		return err
	}
	for _, rb := range rbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rbToUpdate, updateErr := p.rbClient.Get(rb.Namespace, rb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if rbToUpdate.Labels == nil {
				rbToUpdate.Labels = make(map[string]string)
			}
			rbToUpdate.Labels[rtbOwnerLabel] = bindingLabel
			rbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := p.rbClient.Update(rbToUpdate)
			return err
		})
		returnErr = errors.Join(returnErr, retryErr)
	}

	if returnErr != nil {
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		prtbToUpdate, updateErr := p.prtbClient.GetNamespaced(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if prtbToUpdate.Labels == nil {
			prtbToUpdate.Labels = make(map[string]string)
		}
		prtbToUpdate.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := p.prtbClient.Update(prtbToUpdate)
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
