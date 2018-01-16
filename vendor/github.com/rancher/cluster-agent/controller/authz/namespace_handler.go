package authz

import (
	"fmt"

	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	projectNSGetClusterRoleNameFmt = "%v-namespaces"
	projectNSAnn                   = "authz.cluster.auth.io/project-namespaces"
)

func newNamespaceLifecycle(m *manager) *nsLifecycle {
	return &nsLifecycle{m: m}
}

type nsLifecycle struct {
	m *manager
}

func (n *nsLifecycle) Create(obj *v1.Namespace) (*v1.Namespace, error) {
	err := n.syncNS(obj)
	return obj, err
}

func (n *nsLifecycle) Updated(obj *v1.Namespace) (*v1.Namespace, error) {
	err := n.syncNS(obj)
	return obj, err
}

func (n *nsLifecycle) Remove(obj *v1.Namespace) (*v1.Namespace, error) {
	err := n.reconcileNamespaceProjectClusterRole(obj)
	return obj, err
}

func (n *nsLifecycle) syncNS(obj *v1.Namespace) error {
	if err := n.ensureDefaultNamespaceAssigned(obj); err != nil {
		return err
	}

	if err := n.ensurePRTBAddToNamespace(obj); err != nil {
		return err
	}

	return n.reconcileNamespaceProjectClusterRole(obj)
}

func (n *nsLifecycle) ensureDefaultNamespaceAssigned(ns *v1.Namespace) error {
	if ns.Name != "default" {
		return nil
	}

	cluster, err := n.m.clusterLister.Get("", n.m.clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("couldn't find cluster %v", n.m.clusterName)
	}

	updateCluster := false
	c, err := v3.ClusterConditionDefaultNamespaceAssigned.DoUntilTrue(cluster.DeepCopy(), func() (runtime.Object, error) {
		updateCluster = true
		projectID := ns.Annotations[projectIDAnnotation]
		if projectID != "" {
			return nil, nil
		}

		ns = ns.DeepCopy()
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:rancher-default", n.m.clusterName)
		if _, err := n.m.workload.Core.Namespaces(n.m.clusterName).Update(ns); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if updateCluster {
		if _, err := n.m.workload.Management.Management.Clusters("").ObjectClient().Update(cluster.Name, c); err != nil {
			return err
		}
	}
	return err
}

func (n *nsLifecycle) ensurePRTBAddToNamespace(obj *v1.Namespace) error {
	// Get project that contain this namespace
	projectID := obj.Annotations[projectIDAnnotation]
	if len(projectID) == 0 {
		return nil
	}

	prtbs, err := n.m.prtbIndexer.ByIndex(prtbByProjectIndex, projectID)
	if err != nil {
		return errors.Wrapf(err, "couldn't get project role binding templates associated with project id %s", projectID)
	}
	for _, prtb := range prtbs {
		prtb, ok := prtb.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			return errors.Wrapf(err, "object %v is not valid project role template binding", prtb)
		}

		if prtb.RoleTemplateName == "" {
			logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", prtb.Name)
			continue
		}

		rt, err := n.m.rtLister.Get("", prtb.RoleTemplateName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get role template %v", prtb.RoleTemplateName)
		}

		roles := map[string]*v3.RoleTemplate{}
		if err := n.m.gatherRoles(rt, roles); err != nil {
			return err
		}

		if err := n.m.ensureRoles(roles); err != nil {
			return errors.Wrap(err, "couldn't ensure roles")
		}

		for _, role := range roles {
			if err := n.m.ensureBinding(obj.Name, role.Name, prtb); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v %v in %v", role.Name, prtb.Subject.Name, obj.Name)
			}
		}
	}
	return nil
}

// To ensure that all users in a project can do a GET on the namespaces in that project, this
// function ensures that a ClusterRole exists for the project that grants get access to the
// namespaces in the project. A corresponding PRTB handler will ensure that a binding to this
// ClusterRole exists for every project member
func (n *nsLifecycle) reconcileNamespaceProjectClusterRole(ns *v1.Namespace) error {
	var desiredRole string
	if ns.DeletionTimestamp == nil {
		if parts := strings.SplitN(ns.Annotations[projectIDAnnotation], ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
			desiredRole = fmt.Sprintf(projectNSGetClusterRoleNameFmt, parts[1])
		}
	}

	clusterRoles, err := n.m.crIndexer.ByIndex(crByNSIndex, ns.Name)
	if err != nil {
		return err
	}

	roleCli := n.m.workload.K8sClient.RbacV1().ClusterRoles()
	nsInDesiredRole := false
	for _, c := range clusterRoles {
		cr, ok := c.(*rbacv1.ClusterRole)
		if !ok {
			return errors.Errorf("%v is not a ClusterRole", c)
		}

		if cr.Name == desiredRole {
			nsInDesiredRole = true
			continue
		}

		// This ClusterRole has a reference to the namespace, but is not the desired role. Namespace has been moved; remove it from this ClusterRole
		undesiredRole := cr.DeepCopy()
		modified := false
		for i := range undesiredRole.Rules {
			r := &undesiredRole.Rules[i]
			if slice.ContainsString(r.Verbs, "get") && slice.ContainsString(r.Resources, "namespaces") && slice.ContainsString(r.ResourceNames, ns.Name) {
				modified = true
				resNames := r.ResourceNames
				for i := len(resNames) - 1; i >= 0; i-- {
					if resNames[i] == ns.Name {
						resNames = append(resNames[:i], resNames[i+1:]...)
					}
				}
				r.ResourceNames = resNames
			}
		}
		if modified {
			if _, err = roleCli.Update(undesiredRole); err != nil {
				return err
			}
		}
	}

	if !nsInDesiredRole && desiredRole != "" {
		mustUpdate := true
		cr, err := n.m.crLister.Get("", desiredRole)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		// Create new role
		if cr == nil {
			return n.m.createProjectNSRole(desiredRole, ns.Name)
		}

		// Check to see if retrieved role has the namespace (small chance cache could have been updated)
		for _, r := range cr.Rules {
			if slice.ContainsString(r.Verbs, "get") && slice.ContainsString(r.Resources, "namespaces") && slice.ContainsString(r.ResourceNames, ns.Name) {
				// ns already in the role, nothing to do
				mustUpdate = false
			}
		}
		if mustUpdate {
			cr = cr.DeepCopy()
			appendedToExisting := false
			for i := range cr.Rules {
				r := &cr.Rules[i]
				if slice.ContainsString(r.Verbs, "get") && slice.ContainsString(r.Resources, "namespaces") {
					r.ResourceNames = append(r.ResourceNames, ns.Name)
					appendedToExisting = true
					break
				}
			}

			if !appendedToExisting {
				cr.Rules = append(cr.Rules, rbacv1.PolicyRule{
					APIGroups:     []string{""},
					Verbs:         []string{"get"},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{ns.Name},
				})
			}

			_, err = roleCli.Update(cr)
			return err
		}
	}

	return nil
}

func (m *manager) createProjectNSRole(roleName, ns string) error {
	roleCli := m.workload.K8sClient.RbacV1().ClusterRoles()

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        roleName,
			Annotations: map[string]string{projectNSAnn: roleName},
		},
	}
	if ns != "" {
		cr.Rules = []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Verbs:         []string{"get"},
				Resources:     []string{"namespaces"},
				ResourceNames: []string{ns},
			},
		}
	}
	_, err := roleCli.Create(cr)
	return err
}

func crByNS(obj interface{}) ([]string, error) {
	cr, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return []string{}, nil
	}

	if _, ok := cr.Annotations[projectNSAnn]; !ok {
		return []string{}, nil
	}

	var result []string
	for _, r := range cr.Rules {
		if slice.ContainsString(r.Resources, "namespaces") && slice.ContainsString(r.Verbs, "get") {
			result = append(result, r.ResourceNames...)
		}
	}
	return result, nil
}
