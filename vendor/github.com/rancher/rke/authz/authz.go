package authz

import (
	"github.com/rancher/rke/k8s"
	"github.com/sirupsen/logrus"
)

func ApplyJobDeployerServiceAccount(kubeConfigPath string) error {
	logrus.Infof("[authz] Creating rke-job-deployer ServiceAccount")
	k8sClient, err := k8s.NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	if err := k8s.UpdateClusterRoleBindingFromYaml(k8sClient, jobDeployerClusterRoleBinding); err != nil {
		return err
	}
	if err := k8s.UpdateServiceAccountFromYaml(k8sClient, jobDeployerServiceAccount); err != nil {
		return err
	}
	logrus.Infof("[authz] rke-job-deployer ServiceAccount created successfully")
	return nil
}

func ApplySystemNodeClusterRoleBinding(kubeConfigPath string) error {
	logrus.Infof("[authz] Creating system:node ClusterRoleBinding")
	k8sClient, err := k8s.NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	if err := k8s.UpdateClusterRoleBindingFromYaml(k8sClient, systemNodeClusterRoleBinding); err != nil {
		return err
	}
	logrus.Infof("[authz] system:node ClusterRoleBinding created successfully")
	return nil
}
