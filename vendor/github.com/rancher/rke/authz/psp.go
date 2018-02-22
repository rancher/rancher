package authz

import (
	"context"

	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/templates"
)

func ApplyDefaultPodSecurityPolicy(ctx context.Context, kubeConfigPath string, k8sWrapTransport k8s.WrapTransport) error {
	log.Infof(ctx, "[authz] Applying default PodSecurityPolicy")
	k8sClient, err := k8s.NewClient(kubeConfigPath, k8sWrapTransport)
	if err != nil {
		return err
	}
	if err := k8s.UpdatePodSecurityPolicyFromYaml(k8sClient, templates.DefaultPodSecurityPolicy); err != nil {
		return err
	}
	log.Infof(ctx, "[authz] Default PodSecurityPolicy applied successfully")
	return nil
}

func ApplyDefaultPodSecurityPolicyRole(ctx context.Context, kubeConfigPath string, k8sWrapTransport k8s.WrapTransport) error {
	log.Infof(ctx, "[authz] Applying default PodSecurityPolicy Role and RoleBinding")
	k8sClient, err := k8s.NewClient(kubeConfigPath, k8sWrapTransport)
	if err != nil {
		return err
	}
	if err := k8s.UpdateRoleFromYaml(k8sClient, templates.DefaultPodSecurityRole); err != nil {
		return err
	}
	if err := k8s.UpdateRoleBindingFromYaml(k8sClient, templates.DefaultPodSecurityRoleBinding); err != nil {
		return err
	}
	log.Infof(ctx, "[authz] Default PodSecurityPolicy Role and RoleBinding applied successfully")
	return nil
}
