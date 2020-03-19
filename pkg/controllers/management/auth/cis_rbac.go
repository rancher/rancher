package auth

import (
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/rbac"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cisReadOnlyRoleName = "cis-readonly"
)

type cisScanRBACHandler struct {
	roles             rbacv1.RoleInterface
	roleLister        rbacv1.RoleLister
	roleBindings      rbacv1.RoleBindingInterface
	roleBindingLister rbacv1.RoleBindingLister
}

func (rh *cisScanRBACHandler) ensureCISScanBinding(crtb *v3.ClusterRoleTemplateBinding, isOwnerRole bool) error {
	if crtb == nil {
		return nil
	}
	if !isOwnerRole || crtb.ClusterName == "local" {
		return nil
	}
	logrus.Debugf("cisScanRBACHandler: handle crtb: %+v", crtb)

	if err := rh.createRoles(); err != nil {
		return err
	}
	if err := rh.createRoleBindings(crtb); err != nil {
		return err
	}
	return nil
}

func (rh *cisScanRBACHandler) removeCISScanBinding(crtb *v3.ClusterRoleTemplateBinding) error {
	logrus.Debugf("cisScanRBACHandler: handleDelete: crtb: %+v", crtb)
	rbName, err := getRoleBindingName(crtb)
	if err != nil {
		return err
	}
	err = rh.roleBindings.Delete(rbName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	logrus.Debugf("cisScanRBACHandler: handleDelete: deleting cis rb: %v for cluster: %v", rbName, crtb.ClusterName)
	return nil
}

func (rh *cisScanRBACHandler) createRoles() error {
	_, err := rh.roleLister.Get(namespace.GlobalNamespace, cisReadOnlyRoleName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			rule := v1.PolicyRule{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"cisconfigs", "cisbenchmarkversions"},
			}
			role := &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cisReadOnlyRoleName,
					Namespace: namespace.GlobalNamespace,
				},
				Rules: []v1.PolicyRule{rule},
			}
			_, err = rh.roles.Create(role)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (rh *cisScanRBACHandler) createRoleBindings(crtb *v3.ClusterRoleTemplateBinding) error {
	rbName, err := getRoleBindingName(crtb)
	if err != nil {
		return err
	}
	_, err = rh.roleBindingLister.Get(namespace.GlobalNamespace, rbName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			subject, err := rbac.BuildSubjectFromRTB(crtb)
			if err != nil {
				return err
			}
			rb := &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rbName,
					Namespace: namespace.GlobalNamespace,
				},
				Subjects: []v1.Subject{subject},
				RoleRef: v1.RoleRef{
					Kind: "Role",
					Name: cisReadOnlyRoleName,
				},
			}
			_, err = rh.roleBindings.Create(rb)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			logrus.Debugf("cisScanRBACHandler: createRoleBindings: creating rb of subject: %v for cluster: %v", subject.Name, crtb.ClusterName)
		} else {
			return err
		}
	}
	return nil
}

func getRoleBindingName(crtb *v3.ClusterRoleTemplateBinding) (string, error) {
	rbName := crtb.Name + "-" + crtb.Kind + "-" + cisReadOnlyRoleName
	return rbName, nil
}
