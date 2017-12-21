package k8s

import (
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

func UpdateRoleBindingFromYaml(k8sClient *kubernetes.Clientset, roleBindingYaml string) error {
	roleBinding := rbacv1.RoleBinding{}
	err := decodeYamlResource(&roleBinding, roleBindingYaml)
	if err != nil {
		return err
	}

	for retries := 0; retries <= 5; retries++ {
		if err = updateRoleBinding(k8sClient, roleBinding); err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		return nil
	}
	return err
}

func updateRoleBinding(k8sClient *kubernetes.Clientset, roleBinding rbacv1.RoleBinding) error {
	if _, err := k8sClient.RbacV1().RoleBindings(roleBinding.Namespace).Create(&roleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.RbacV1().RoleBindings(roleBinding.Namespace).Update(&roleBinding); err != nil {
			return err
		}
	}
	return nil
}

func UpdateRoleFromYaml(k8sClient *kubernetes.Clientset, roleYaml string) error {
	role := rbacv1.Role{}
	err := decodeYamlResource(&role, roleYaml)
	if err != nil {
		return err
	}

	for retries := 0; retries <= 5; retries++ {
		if err = updateRole(k8sClient, role); err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		return nil
	}
	return err
}

func updateRole(k8sClient *kubernetes.Clientset, role rbacv1.Role) error {
	if _, err := k8sClient.RbacV1().Roles(role.Namespace).Create(&role); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.RbacV1().Roles(role.Namespace).Update(&role); err != nil {
			return err
		}
	}
	return nil
}
