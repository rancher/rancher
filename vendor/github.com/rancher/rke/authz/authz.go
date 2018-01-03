package authz

import (
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/templates"
	"github.com/sirupsen/logrus"
)

func ApplyJobDeployerServiceAccount(kubeConfigPath string) error {
	logrus.Infof("[authz] Creating rke-job-deployer ServiceAccount")
	k8sClient, err := k8s.NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	if err := k8s.UpdateClusterRoleBindingFromYaml(k8sClient, templates.JobDeployerClusterRoleBinding); err != nil {
		return err
	}
	if err := k8s.UpdateServiceAccountFromYaml(k8sClient, templates.JobDeployerServiceAccount); err != nil {
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
	if err := k8s.UpdateClusterRoleBindingFromYaml(k8sClient, templates.SystemNodeClusterRoleBinding); err != nil {
		return err
	}
	logrus.Infof("[authz] system:node ClusterRoleBinding created successfully")
	return nil
}
