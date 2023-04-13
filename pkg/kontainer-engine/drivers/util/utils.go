package util

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	rketypes "github.com/rancher/rke/types"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultNamespace          = "default"
	cattleNamespace           = "cattle-system"
	clusterAdmin              = "cluster-admin"
	netesDefault              = "netes-default"
	kontainerEngine           = "kontainer-engine"
	oldClusterRoleBindingName = "netes-default-clusterRoleBinding"
	newClusterRoleBindingName = "system-netes-default-clusterRoleBinding"
)

// GenerateServiceAccountToken generate a serviceAccountToken for clusterAdmin given a rest clientset
func GenerateServiceAccountToken(clientset kubernetes.Interface) (string, error) {
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: cattleNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", err
	}

	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: kontainerEngine,
		},
	}

	_, err = clientset.CoreV1().ServiceAccounts(cattleNamespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating service account: %v", err)
	}

	adminRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdmin,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
		},
	}
	clusterAdminRole, err := clientset.RbacV1().ClusterRoles().Get(context.TODO(), clusterAdmin, metav1.GetOptions{})
	if err != nil {
		clusterAdminRole, err = clientset.RbacV1().ClusterRoles().Create(context.TODO(), adminRole, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("error creating admin role: %v", err)
		}
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: newClusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: cattleNamespace,
				APIGroup:  v1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterAdminRole.Name,
			APIGroup: rbacv1.GroupName,
		},
	}
	if _, err = clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating role bindings: %v", err)
	}

	if serviceAccount, err = clientset.CoreV1().ServiceAccounts(cattleNamespace).Get(context.Background(), serviceAccount.Name, metav1.GetOptions{}); err != nil {
		return "", fmt.Errorf("error getting service account: %w", err)
	}
	secret, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), nil, clientset, serviceAccount)
	if err != nil {
		return "", fmt.Errorf("error ensuring secret for service account: %w", err)
	}
	return string(secret.Data["token"]), nil
}

func DeleteLegacyServiceAccountAndRoleBinding(clientset kubernetes.Interface) error {
	_, err := clientset.CoreV1().ServiceAccounts(defaultNamespace).Get(context.TODO(), netesDefault, metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		err = clientset.CoreV1().ServiceAccounts(defaultNamespace).Delete(context.TODO(), netesDefault, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	_, err = clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), oldClusterRoleBindingName, metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		err = clientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), oldClusterRoleBindingName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func ConvertToRkeConfig(config string) (rketypes.RancherKubernetesEngineConfig, error) {
	var rkeConfig rketypes.RancherKubernetesEngineConfig
	if err := yaml.Unmarshal([]byte(config), &rkeConfig); err != nil {
		return rkeConfig, err
	}
	return rkeConfig, nil
}
