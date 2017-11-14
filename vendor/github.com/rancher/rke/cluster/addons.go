package cluster

import (
	"fmt"
	"time"

	"github.com/rancher/rke/addons"
	"github.com/rancher/rke/k8s"
	"github.com/sirupsen/logrus"
)

const (
	KubeDNSAddonResourceName = "rke-kubedns-addon"
	UserAddonResourceName    = "rke-user-addon"
)

func (c *Cluster) DeployK8sAddOns() error {
	err := c.deployKubeDNS()
	return err
}

func (c *Cluster) DeployUserAddOns() error {
	logrus.Infof("[addons] Setting up user addons..")
	if c.Addons == "" {
		logrus.Infof("[addons] No user addons configured..")
		return nil
	}

	if err := c.doAddonDeploy(c.Addons, UserAddonResourceName); err != nil {
		return err
	}
	logrus.Infof("[addons] User addon deployed successfully..")
	return nil

}

func (c *Cluster) deployKubeDNS() error {
	logrus.Infof("[addons] Setting up KubeDNS")
	kubeDNSConfig := map[string]string{
		addons.KubeDNSServer:          c.ClusterDNSServer,
		addons.KubeDNSClusterDomain:   c.ClusterDomain,
		addons.KubeDNSImage:           c.SystemImages[KubeDNSImage],
		addons.DNSMasqImage:           c.SystemImages[DNSMasqImage],
		addons.KubeDNSSidecarImage:    c.SystemImages[KubeDNSSidecarImage],
		addons.KubeDNSAutoScalerImage: c.SystemImages[KubeDNSAutoScalerImage],
	}
	kubeDNSYaml := addons.GetKubeDNSManifest(kubeDNSConfig)
	if err := c.doAddonDeploy(kubeDNSYaml, KubeDNSAddonResourceName); err != nil {
		return err
	}
	logrus.Infof("[addons] KubeDNS deployed successfully..")
	return nil

}

func (c *Cluster) doAddonDeploy(addonYaml, resourceName string) error {

	err := c.StoreAddonConfigMap(addonYaml, resourceName)
	if err != nil {
		return fmt.Errorf("Failed to save addon ConfigMap: %v", err)
	}

	logrus.Infof("[addons] Executing deploy job..")

	addonJob := addons.GetAddonsExcuteJob(resourceName, c.ControlPlaneHosts[0].HostnameOverride, c.Services.KubeAPI.Image)
	err = c.ApplySystemAddonExcuteJob(addonJob)
	if err != nil {
		return fmt.Errorf("Failed to deploy addon execute job: %v", err)
	}
	return nil
}

func (c *Cluster) StoreAddonConfigMap(addonYaml string, addonName string) error {
	logrus.Infof("[addons] Saving addon ConfigMap to Kubernetes")
	kubeClient, err := k8s.NewClient(c.LocalKubeConfigPath)
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
			logrus.Infof("[addons] Successfully Saved addon to Kubernetes ConfigMap: %s", addonName)
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

	if err := k8s.ApplyK8sSystemJob(addonJob, c.LocalKubeConfigPath); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
