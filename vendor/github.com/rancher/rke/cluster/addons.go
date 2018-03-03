package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rancher/rke/addons"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
)

const (
	KubeDNSAddonResourceName = "rke-kubedns-addon"
	UserAddonResourceName    = "rke-user-addon"
	IngressAddonResourceName = "rke-ingress-controller"
)

type ingressOptions struct {
	RBACConfig     string
	Options        map[string]string
	NodeSelector   map[string]string
	AlpineImage    string
	IngressImage   string
	IngressBackend string
}

func (c *Cluster) deployK8sAddOns(ctx context.Context) error {
	if err := c.deployKubeDNS(ctx); err != nil {
		return err
	}
	return c.deployIngress(ctx)
}

func (c *Cluster) deployUserAddOns(ctx context.Context) error {
	log.Infof(ctx, "[addons] Setting up user addons..")
	if c.Addons == "" {
		log.Infof(ctx, "[addons] No user addons configured..")
		return nil
	}

	if err := c.doAddonDeploy(ctx, c.Addons, UserAddonResourceName); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] User addon deployed successfully..")
	return nil

}

func (c *Cluster) deployKubeDNS(ctx context.Context) error {
	log.Infof(ctx, "[addons] Setting up KubeDNS")
	kubeDNSConfig := map[string]string{
		addons.KubeDNSServer:          c.ClusterDNSServer,
		addons.KubeDNSClusterDomain:   c.ClusterDomain,
		addons.KubeDNSImage:           c.SystemImages.KubeDNS,
		addons.DNSMasqImage:           c.SystemImages.DNSmasq,
		addons.KubeDNSSidecarImage:    c.SystemImages.KubeDNSSidecar,
		addons.KubeDNSAutoScalerImage: c.SystemImages.KubeDNSAutoscaler,
	}
	kubeDNSYaml, err := addons.GetKubeDNSManifest(kubeDNSConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, kubeDNSYaml, KubeDNSAddonResourceName); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] KubeDNS deployed successfully..")
	return nil
}

func (c *Cluster) deployWithKubectl(ctx context.Context, addonYaml string) error {
	buf := bytes.NewBufferString(addonYaml)
	cmd := exec.Command("kubectl", "--kubeconfig", c.LocalKubeConfigPath, "apply", "-f", "-")
	cmd.Stdin = buf
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Cluster) doAddonDeploy(ctx context.Context, addonYaml, resourceName string) error {
	if c.UseKubectlDeploy {
		return c.deployWithKubectl(ctx, addonYaml)
	}

	err := c.StoreAddonConfigMap(ctx, addonYaml, resourceName)
	if err != nil {
		return fmt.Errorf("Failed to save addon ConfigMap: %v", err)
	}

	log.Infof(ctx, "[addons] Executing deploy job..")

	addonJob, err := addons.GetAddonsExcuteJob(resourceName, c.ControlPlaneHosts[0].HostnameOverride, c.Services.KubeAPI.Image)
	if err != nil {
		return fmt.Errorf("Failed to deploy addon execute job: %v", err)
	}
	err = c.ApplySystemAddonExcuteJob(addonJob)
	if err != nil {
		return fmt.Errorf("Failed to deploy addon execute job: %v", err)
	}
	return nil
}

func (c *Cluster) StoreAddonConfigMap(ctx context.Context, addonYaml string, addonName string) error {
	log.Infof(ctx, "[addons] Saving addon ConfigMap to Kubernetes")
	kubeClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return err
	}
	timeout := make(chan bool, 1)
	go func() {
		for {
			err := k8s.UpdateConfigMap(kubeClient, []byte(addonYaml), addonName)
			if err != nil {
				time.Sleep(time.Second * 5)
				fmt.Println(err)
				continue
			}
			log.Infof(ctx, "[addons] Successfully Saved addon to Kubernetes ConfigMap: %s", addonName)
			timeout <- true
			break
		}
	}()
	select {
	case <-timeout:
		return nil
	case <-time.After(time.Second * UpdateStateTimeout):
		return fmt.Errorf("[addons] Timeout waiting for kubernetes to be ready")
	}
}

func (c *Cluster) ApplySystemAddonExcuteJob(addonJob string) error {

	if err := k8s.ApplyK8sSystemJob(addonJob, c.LocalKubeConfigPath, c.K8sWrapTransport); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (c *Cluster) deployIngress(ctx context.Context) error {
	if c.Ingress.Provider == "none" {
		log.Infof(ctx, "[ingress] ingress controller is not defined, skipping ingress controller")
		return nil
	}
	log.Infof(ctx, "[ingress] Setting up %s ingress controller", c.Ingress.Provider)
	ingressConfig := ingressOptions{
		RBACConfig:     c.Authorization.Mode,
		Options:        c.Ingress.Options,
		NodeSelector:   c.Ingress.NodeSelector,
		AlpineImage:    c.SystemImages.Alpine,
		IngressImage:   c.SystemImages.Ingress,
		IngressBackend: c.SystemImages.IngressBackend,
	}
	// Currently only deploying nginx ingress controller
	ingressYaml, err := addons.GetNginxIngressManifest(ingressConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, ingressYaml, IngressAddonResourceName); err != nil {
		return err
	}
	log.Infof(ctx, "[ingress] ingress controller %s is successfully deployed", c.Ingress.Provider)
	return nil
}
