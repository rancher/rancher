package rbac

import (
	"fmt"

	"github.com/rancher/norman/types/slice"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (m *manager) reconcileProjectAccessToGlobalResources(binding *v3.ProjectRoleTemplateBinding, roles []string) (map[string]bool, error) {
	if len(roles) == 0 {
		return nil, nil
	}

	bindingCli := m.workload.RBAC.ClusterRoleBindings("")

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

	return crbsToKeep, nil
}

// EnsureGlobalResourcesRolesForPRTB ensures that all necessary roles exist and contain the rules needed to
// enforce permissions described by RoleTemplate rules. A slice of strings indicating role names is returned.
func (m *manager) ensureGlobalResourcesRolesForPRTB(projectName string, rts map[string]*v3.RoleTemplate) ([]string, error) {
	roles := sets.New[string]()

	if projectName == "" {
		return nil, nil
	}

	var roleVerb, roleSuffix string
	for _, r := range rts {
		for _, rule := range r.Rules {
			hasNamespaceResources := slice.ContainsString(rule.Resources, "namespaces") || slice.ContainsString(rule.Resources, "*")
			hasNamespaceGroup := slice.ContainsString(rule.APIGroups, "") || slice.ContainsString(rule.APIGroups, "*")
			if hasNamespaceGroup && hasNamespaceResources && len(rule.ResourceNames) == 0 {
				if slice.ContainsString(rule.Verbs, "*") || slice.ContainsString(rule.Verbs, "create") {
					roleVerb = "*"
					roles.Insert("create-ns")
					if nsRole, _ := m.crLister.Get("", "create-ns"); nsRole == nil {
						createNSRT, err := m.rtLister.Get("", "create-ns")
						if err != nil {
							return nil, err
						}
						if err := m.ensureRoles(map[string]*v3.RoleTemplate{"create-ns": createNSRT}); err != nil && !apierrors.IsAlreadyExists(err) {
							return nil, err
						}
					}
					break
				}
			}

		}
	}
	if roleVerb == "" {
		roleVerb = "get"
	}
	roleSuffix = projectNSVerbToSuffix[roleVerb]
	role := fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, roleSuffix)
	roles.Insert(role)

	for _, rt := range rts {
		for resource, baseRule := range globalResourceRulesNeededInProjects {
			verbs, err := m.checkForGlobalResourceRules(rt, resource, baseRule)
			if err != nil {
				return nil, err
			}

			roleName, err := m.reconcileRoleForProjectAccessToGlobalResource(resource, rt.Name, verbs, baseRule)
			if err != nil {
				return nil, err
			}

			// if a role was created or updated append it to the existing roles
			if roleName != "" {
				roles.Insert(roleName)
			}
		}
	}

	return sets.List(roles), nil
}
