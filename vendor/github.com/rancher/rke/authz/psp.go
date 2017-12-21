package authz

import (
	"github.com/rancher/rke/k8s"
	"github.com/sirupsen/logrus"
)

func ApplyDefaultPodSecurityPolicy(kubeConfigPath string) error {
	logrus.Infof("[authz] Applying default PodSecurityPolicy")
	k8sClient, err := k8s.NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	if err := k8s.UpdatePodSecurityPolicyFromYaml(k8sClient, DefaultPodSecurityPolicy); err != nil {
		return err
	}
	logrus.Infof("[authz] Default PodSecurityPolicy applied successfully")
	return nil
}

func ApplyDefaultPodSecurityPolicyRole(kubeConfigPath string) error {
	logrus.Infof("[authz] Applying default PodSecurityPolicy Role and RoleBinding")
	k8sClient, err := k8s.NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	if err := k8s.UpdateRoleFromYaml(k8sClient, DefaultPodSecurityRole); err != nil {
		return err
	}
	if err := k8s.UpdateRoleBindingFromYaml(k8sClient, DefaultPodSecurityRoleBinding); err != nil {
		return err
	}
	logrus.Infof("[authz] Default PodSecurityPolicy Role and RoleBinding applied successfully")
	return nil
}
