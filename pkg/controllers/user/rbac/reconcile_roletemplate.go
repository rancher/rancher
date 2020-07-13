package rbac

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types/slice"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *manager) reconcileProjectAccessToGlobalResources(binding *v3.ProjectRoleTemplateBinding, rts map[string]*v3.RoleTemplate) (map[string]bool, error) {
	var role string
	var createNSPerms bool
	var roles []string
	if parts := strings.SplitN(binding.ProjectName, ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
		projectName := parts[1]
		var roleVerb, roleSuffix string
		for _, r := range rts {
			for _, rule := range r.Rules {
				if slice.ContainsString(rule.Resources, "namespaces") && len(rule.ResourceNames) == 0 {
					if slice.ContainsString(rule.Verbs, "*") || slice.ContainsString(rule.Verbs, "create") {
						roleVerb = "*"
						createNSPerms = true
						break
					}
				}

			}
		}
		if roleVerb == "" {
			roleVerb = "get"
		}
		roleSuffix = projectNSVerbToSuffix[roleVerb]
		role = fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, roleSuffix)
		roles = append(roles, role)

		for _, rt := range rts {
			for resource := range globalResourcesNeededInProjects {
				verbs, err := m.checkForGlobalResourceRules(rt, resource)
				if err != nil {
					return nil, err
				}
				if len(verbs) > 0 {
					roleName, err := m.reconcileRoleForProjectAccessToGlobalResource(resource, rt, verbs)
					if err != nil {
						return nil, err
					}
					roles = append(roles, roleName)
				}
			}
		}
	}

	if len(roles) == 0 {
		return nil, nil
	}

	bindingCli := m.workload.RBAC.ClusterRoleBindings("")

	if createNSPerms {
		roles = append(roles, "create-ns")
		if nsRole, _ := m.crLister.Get("", "create-ns"); nsRole == nil {
			createNSRT, err := m.rtLister.Get("", "create-ns")
			if err != nil {
				return nil, err
			}
			if err := m.ensureRoles(map[string]*v3.RoleTemplate{"create-ns": createNSRT}); err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
		}
	}

	rtbUID := string(binding.UID)
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
			createdCRB, err := bindingCli.Create(&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "clusterrolebinding-",
					Labels: map[string]string{
						rtbUID: owner,
					},
				},
				Subjects: []rbacv1.Subject{subject},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: role,
				},
			})
			if err != nil {
				return nil, err
			}
			crbsToKeep[createdCRB.Name] = true
		} else {
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

	}
	return crbsToKeep, nil
}
