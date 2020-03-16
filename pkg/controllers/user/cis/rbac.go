package cis

import (
	"fmt"

	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func (rh *cisScanRBACHandler) Sync(key string, crtb *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	if crtb == nil {
		return nil, nil
	}
	if crtb.DeletionTimestamp != nil {
		return rh.handleDelete(key, crtb)
	}
	if crtb.RoleTemplateName != "cluster-owner" || crtb.ClusterName == "local" {
		return nil, nil
	}
	logrus.Debugf("cisScanRBACHandler: Sync: crtb: %+v", crtb)

	if err := rh.createRoles(); err != nil {
		return nil, err
	}
	if err := rh.createRoleBindings(crtb); err != nil {
		return nil, err
	}
	return nil, nil
}

func (rh *cisScanRBACHandler) handleDelete(_ string, crtb *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	logrus.Debugf("cisScanRBACHandler: handleDelete: crtb: %+v", crtb)
	rbName, err := getRoleBindingName(crtb)
	if err != nil {
		return nil, err
	}
	err = rh.roleBindings.Delete(rbName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	logrus.Debugf("cisScanRBACHandler: handleDelete: deleting rb of user: %v for cluster: %v", crtb.UserName, crtb.ClusterName)
	return nil, nil
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
			subject := v1.Subject{
				Kind: "User",
				Name: crtb.UserName,
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
			_, err := rh.roleBindings.Create(rb)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			logrus.Debugf("cisScanRBACHandler: createRoleBindings: creating rb of user: %v for cluster: %v", crtb.UserName, crtb.ClusterName)
		} else {
			return err
		}
	}
	return nil
}

func getRoleBindingName(crtb *v3.ClusterRoleTemplateBinding) (string, error) {
	rbName := ""
	if crtb.UserName == "" {
		return rbName, fmt.Errorf("username not found in crtb: %v", crtb.Name)
	}
	if crtb.ClusterName == "" {
		return rbName, fmt.Errorf("cluster name not found in crtb: %v", crtb.Name)
	}
	rbName = crtb.UserName + "-" + crtb.ClusterName + "-" + cisReadOnlyRoleName
	return rbName, nil
}
