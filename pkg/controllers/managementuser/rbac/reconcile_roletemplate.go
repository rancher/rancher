package rbac

import (
	"fmt"
	"maps"
	"slices"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac/roletemplates"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (m *manager) reconcileProjectAccessToGlobalResources(binding *v3.ProjectRoleTemplateBinding, roles []string) (map[string]bool, error) {
	if len(roles) == 0 {
		return nil, nil
	}

	var bindingCli wrbacv1.ClusterRoleBindingClient = m.workload.RBACw.ClusterRoleBinding()

	rtbUID := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		return nil, err
	}
	crbsToKeep := make(map[string]bool)
	for _, role := range roles {
		crbKey := rbRoleSubjectKey(role, subject)
		crbs, _ := m.crbIndexer.ByIndex(crbByRoleAndSubjectIndex, crbKey)
		if len(crbs) == 0 {
			logrus.Infof("Creating clusterRoleBinding for project access to global resource for subject %v role %v.", subject.Name, role)
			roleRef := rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: role,
			}
			crbName := pkgrbac.NameForClusterRoleBinding(roleRef, subject)
			createdCRB, err := bindingCli.Create(&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: crbName,
					Labels: map[string]string{
						rtbUID: owner,
					},
				},
				Subjects: []rbacv1.Subject{subject},
				RoleRef:  roleRef,
			})
			if err == nil {
				crbsToKeep[createdCRB.Name] = true
				continue
			}
			if !apierrors.IsAlreadyExists(err) {
				return nil, err
			}

			// the binding exists but was not found in the index, manually retrieve it so that we can add appropriate labels
			crb, err := bindingCli.Get(crbName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			crbs = append(crbs, crb)
		}

	CRBs:
		for _, obj := range crbs {
			crb, ok := obj.(*rbacv1.ClusterRoleBinding)
			if !ok {
				continue
			}

			crbsToKeep[crb.Name] = true
			for owner := range crb.Labels {
				if rtbUID == owner {
					continue CRBs
				}
			}

			crb = crb.DeepCopy()
			if crb.Labels == nil {
				crb.Labels = map[string]string{}
			}
			crb.Labels[rtbUID] = owner
			logrus.Infof("Updating clusterRoleBinding %v for project access to global resource for subject %v role %v.", crb.Name, subject.Name, role)
			_, err := bindingCli.Update(crb)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := m.removeStaleProjectNamespaceCRBs(binding, crbsToKeep); err != nil {
		return nil, err
	}

	return crbsToKeep, nil
}

// removeStaleProjectNamespaceCRBs removes the binding's ownership from ClusterRoleBindings to project namespace roles
// it no longer grants, such as namespaces-edit for a RoleTemplate that only allows creating namespaces. A
// ClusterRoleBinding is deleted once no other binding owns it.
func (m *manager) removeStaleProjectNamespaceCRBs(binding *v3.ProjectRoleTemplateBinding, crbsToKeep map[string]bool) error {
	projectName := parseProjectName(binding.ProjectName)
	if projectName == "" {
		return nil
	}
	nsRoles := []string{"create-ns"}
	for _, suffix := range append(slices.Collect(maps.Values(projectNSVerbToSuffix)), projectNSManageSuffix) {
		nsRoles = append(nsRoles, fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, suffix))
	}

	rtbNsAndName := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	crbs, err := m.crbLister.List(labels.Set{rtbNsAndName: owner}.AsSelector())
	if err != nil {
		return err
	}
	for _, crb := range crbs {
		if crbsToKeep[crb.Name] || !slices.Contains(nsRoles, crb.RoleRef.Name) {
			continue
		}

		crb = crb.DeepCopy()
		delete(crb.Labels, rtbNsAndName)
		// Bindings created before 2.5 also own the ClusterRoleBinding through a UID label.
		if crb.Labels[string(binding.UID)] == owner {
			delete(crb.Labels, string(binding.UID))
		}

		eligibleForDeletion, err := m.noRemainingOwnerLabels(crb)
		if err != nil {
			return err
		}
		if eligibleForDeletion {
			logrus.Infof("Deleting clusterRoleBinding %v for project access to global resource role %v.", crb.Name, crb.RoleRef.Name)
			if err := m.clusterRoleBindings.Delete(crb.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		crbsToKeep[crb.Name] = true
		if _, err := m.clusterRoleBindings.Update(crb); err != nil {
			return err
		}
	}
	return nil
}

// EnsureGlobalResourcesRolesForPRTB ensures that all necessary roles exist and contain the rules needed to
// enforce permissions described by RoleTemplate rules. A slice of strings indicating role names is returned.
func (m *manager) ensureGlobalResourcesRolesForPRTB(projectName string, rts map[string]*v3.RoleTemplate) ([]string, error) {
	roles := sets.New[string]()

	if projectName == "" {
		return nil, nil
	}

	var allRules []rbacv1.PolicyRule
	for _, rt := range rts {
		// Get the rules of the RoleTemplate
		var rules []rbacv1.PolicyRule
		if rt.External {
			externalRole, err := m.crLister.Get(rt.Name)
			if apierrors.IsNotFound(err) {
				// Don't error if it doesn't exist, just move on to the next RoleTemplate
				continue
			}
			if err != nil {
				return nil, err
			}
			if externalRole != nil {
				rules = externalRole.Rules
			}
		} else {
			rules = rt.Rules
		}
		allRules = append(allRules, rules...)

		// Find any Promoted Rules
		promotedRules := roletemplates.ExtractPromotedRules(rules)

		// Create a ClusterRole with the Promoted Rules
		roleName, err := m.reconcileRoleForProjectAccessToGlobalResource(rt.Name, promotedRules)
		if err != nil {
			return nil, err
		}

		// if a role was created or updated append it to the existing roles
		if roleName != "" {
			roles.Insert(roleName)
		}
	}

	nsRoles := roletemplates.ProjectNamespaceRoles(projectName, allRules)
	if slices.Contains(nsRoles, "create-ns") {
		if nsRole, _ := m.crLister.Get("create-ns"); nsRole == nil {
			createNSRT, err := m.rtLister.Get("create-ns")
			if err != nil {
				return nil, err
			}
			if err := m.ensureRoles(map[string]*v3.RoleTemplate{"create-ns": createNSRT}); err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
		}
	}
	roles.Insert(nsRoles...)

	return sets.List(roles), nil
}
