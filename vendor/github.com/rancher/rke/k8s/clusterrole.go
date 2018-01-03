package k8s

import (
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

func UpdateClusterRoleBindingFromYaml(k8sClient *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{}
	if err := decodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}
	return retryTo(updateClusterRoleBinding, k8sClient, clusterRoleBinding, DefaultRetries, DefaultSleepSeconds)
}

func updateClusterRoleBinding(k8sClient *kubernetes.Clientset, crb interface{}) error {
	clusterRoleBinding := crb.(rbacv1.ClusterRoleBinding)
	if _, err := k8sClient.RbacV1().ClusterRoleBindings().Create(&clusterRoleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.RbacV1().ClusterRoleBindings().Update(&clusterRoleBinding); err != nil {
			return err
		}
	}
	return nil
}

func UpdateClusterRoleFromYaml(k8sClient *kubernetes.Clientset, clusterRoleYaml string) error {
	clusterRole := rbacv1.ClusterRole{}
	if err := decodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}

	return retryTo(updateClusterRole, k8sClient, clusterRole, DefaultRetries, DefaultSleepSeconds)
}

func updateClusterRole(k8sClient *kubernetes.Clientset, cr interface{}) error {
	clusterRole := cr.(rbacv1.ClusterRole)
	if _, err := k8sClient.RbacV1().ClusterRoles().Create(&clusterRole); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.RbacV1().ClusterRoles().Update(&clusterRole); err != nil {
			return err
		}
	}
	return nil
}
