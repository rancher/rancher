package k8s

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func UpdateClusterRoleBindingFromYaml(k8sClient *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{}
	if err := DecodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}
	return retryTo(updateClusterRoleBinding, k8sClient, clusterRoleBinding, DefaultRetries, DefaultSleepSeconds)
}

func updateClusterRoleBinding(k8sClient *kubernetes.Clientset, crb interface{}) error {
	clusterRoleBinding := crb.(rbacv1.ClusterRoleBinding)
	if _, err := k8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), &clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.RbacV1().ClusterRoleBindings().Update(context.TODO(), &clusterRoleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func UpdateClusterRoleFromYaml(k8sClient *kubernetes.Clientset, clusterRoleYaml string) error {
	clusterRole := rbacv1.ClusterRole{}
	if err := DecodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}

	return retryTo(updateClusterRole, k8sClient, clusterRole, DefaultRetries, DefaultSleepSeconds)
}

func updateClusterRole(k8sClient *kubernetes.Clientset, cr interface{}) error {
	clusterRole := cr.(rbacv1.ClusterRole)
	if _, err := k8sClient.RbacV1().ClusterRoles().Create(context.TODO(), &clusterRole, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.RbacV1().ClusterRoles().Update(context.TODO(), &clusterRole, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}
