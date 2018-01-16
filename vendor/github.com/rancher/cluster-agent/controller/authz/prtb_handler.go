package authz

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const owner = "owner"

func newPRTBLifecycle(m *manager) *prtbLifecycle {
	return &prtbLifecycle{m: m}
}

type prtbLifecycle struct {
	m *manager
}

func (p *prtbLifecycle) Create(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	err := p.syncPRTB(obj)
	return obj, err
}

func (p *prtbLifecycle) Updated(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	err := p.syncPRTB(obj)
	return obj, err
}

func (p *prtbLifecycle) Remove(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	err := p.ensurePRTBDelete(obj)
	return obj, err
}

func (p *prtbLifecycle) syncPRTB(binding *v3.ProjectRoleTemplateBinding) error {
	if binding.RoleTemplateName == "" {
		logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.Subject.Name == "" {
		logrus.Warnf("Binding %v has no subject. Skipping", binding.Name)
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
	if len(namespaces) == 0 {
		return nil
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
		for _, role := range roles {
			if err := p.m.ensureBinding(ns.Name, role.Name, binding); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v %v in %v", role.Name, binding.Subject.Name, ns.Name)
			}
		}
	}

	return p.reconcileProjectNSGetAccess(binding)
}

func (p *prtbLifecycle) ensurePRTBDelete(binding *v3.ProjectRoleTemplateBinding) error {
	// Get namespaces belonging to project
	namespaces, err := p.m.nsIndexer.ByIndex(nsByProjectIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		bindingCli := p.m.workload.K8sClient.RbacV1().RoleBindings(ns.Name)
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

	return p.reconcileProjectNSGetAccessForDelete(binding)
}

func (p *prtbLifecycle) reconcileProjectNSGetAccess(binding *v3.ProjectRoleTemplateBinding) error {
	var role string
	if parts := strings.SplitN(binding.ProjectName, ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
		role = fmt.Sprintf(projectNSGetClusterRoleNameFmt, parts[1])
	}

	if role == "" {
		return nil
	}

	bindingCli := p.m.workload.K8sClient.RbacV1().ClusterRoleBindings()
	bindingName := role + "-" + binding.Subject.Name

	rtbUID := string(binding.UID)
	crb, _ := p.m.crbLister.Get("", bindingName)
	if crb == nil {
		_, err := bindingCli.Create(&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: bindingName,
				Labels: map[string]string{
					rtbUID: owner,
				},
			},
			Subjects: []rbacv1.Subject{binding.Subject},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: role,
			},
		})
		return err
	}

	for owner := range crb.Labels {
		if rtbUID == owner {
			return nil
		}
	}

	crb = crb.DeepCopy()
	if crb.Labels == nil {
		crb.Labels = map[string]string{}
	}
	crb.Labels[rtbUID] = owner
	_, err := bindingCli.Update(crb)
	return err
}

func (p *prtbLifecycle) reconcileProjectNSGetAccessForDelete(binding *v3.ProjectRoleTemplateBinding) error {
	var role string
	if parts := strings.SplitN(binding.ProjectName, ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
		role = fmt.Sprintf(projectNSGetClusterRoleNameFmt, parts[1])
	}

	if role == "" {
		return nil
	}

	prtbs, err := p.m.prtbIndexer.ByIndex(prtbByProjectUserIndex, getPRTBProjectAndUserKey(binding))
	if err != nil {
		return err
	}
	if len(prtbs) != 0 {
		for _, p := range prtbs {
			pr := p.(*v3.ProjectRoleTemplateBinding)
			if pr.DeletionTimestamp == nil {
				return nil
			}
		}
	}

	bindingCli := p.m.workload.K8sClient.RbacV1().ClusterRoleBindings()
	rtbUID := string(binding.UID)
	set := labels.Set(map[string]string{rtbUID: owner})
	crbs, err := p.m.crbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}

	for _, crb := range crbs {
		crb = crb.DeepCopy()
		for k, v := range crb.Labels {
			if k == rtbUID && v == owner {
				delete(crb.Labels, k)
			}
		}

		if len(crb.Labels) == 0 {
			if err := bindingCli.Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
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
