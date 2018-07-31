package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"io/ioutil"
	"net/http"
	"strings"

	"github.com/rancher/rke/addons"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	KubeDNSAddonResourceName      = "rke-kubedns-addon"
	UserAddonResourceName         = "rke-user-addon"
	IngressAddonResourceName      = "rke-ingress-controller"
	UserAddonsIncludeResourceName = "rke-user-includes-addons"

	IngressAddonJobName            = "rke-ingress-controller-deploy-job"
	IngressAddonDeleteJobName      = "rke-ingress-controller-delete-job"
	MetricsServerAddonResourceName = "rke-metrics-addon"
)

type ingressOptions struct {
	RBACConfig     string
	Options        map[string]string
	NodeSelector   map[string]string
	ExtraArgs      map[string]string
	AlpineImage    string
	IngressImage   string
	IngressBackend string
}

type MetricsServerOptions struct {
	RBACConfig         string
	Options            map[string]string
	MetricsServerImage string
}

type addonError struct {
	err        string
	isCritical bool
}

func (e *addonError) Error() string {
	return e.err
}

func (c *Cluster) deployK8sAddOns(ctx context.Context) error {
	if err := c.deployKubeDNS(ctx); err != nil {
		if err, ok := err.(*addonError); ok && err.isCritical {
			return err
		}
		log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", KubeDNSAddonResourceName, err)
	}
	if c.Monitoring.Provider == DefaultMonitoringProvider {
		if err := c.deployMetricServer(ctx); err != nil {
			if err, ok := err.(*addonError); ok && err.isCritical {
				return err
			}
			log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", MetricsServerAddonResourceName, err)
		}
	}
	if err := c.deployIngress(ctx); err != nil {
		if err, ok := err.(*addonError); ok && err.isCritical {
			return err
		}
		log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", IngressAddonResourceName, err)

	}
	return nil
}

func (c *Cluster) deployUserAddOns(ctx context.Context) error {
	log.Infof(ctx, "[addons] Setting up user addons")
	if c.Addons != "" {
		if err := c.doAddonDeploy(ctx, c.Addons, UserAddonResourceName, false); err != nil {
			return err
		}
	}
	if len(c.AddonsInclude) > 0 {
		if err := c.deployAddonsInclude(ctx); err != nil {
			return err
		}
	}
	if c.Addons == "" && len(c.AddonsInclude) == 0 {
		log.Infof(ctx, "[addons] no user addons defined")
	} else {
		log.Infof(ctx, "[addons] User addons deployed successfully")
	}
	return nil
}

func (c *Cluster) deployAddonsInclude(ctx context.Context) error {
	var manifests []byte
	log.Infof(ctx, "[addons] Checking for included user addons")

	if len(c.AddonsInclude) == 0 {
		log.Infof(ctx, "[addons] No included addon paths or urls..")
		return nil
	}
	for _, addon := range c.AddonsInclude {
		if strings.HasPrefix(addon, "http") {
			addonYAML, err := getAddonFromURL(addon)
			if err != nil {
				return err
			}
			log.Infof(ctx, "[addons] Adding addon from url %s", addon)
			logrus.Debugf("URL Yaml: %s", addonYAML)

			if err := validateUserAddonYAML(addonYAML); err != nil {
				return err
			}
			manifests = append(manifests, addonYAML...)
		} else if isFilePath(addon) {
			addonYAML, err := ioutil.ReadFile(addon)
			if err != nil {
				return err
			}
			log.Infof(ctx, "[addons] Adding addon from %s", addon)
			logrus.Debugf("FilePath Yaml: %s", string(addonYAML))

			// make sure we properly separated manifests
			addonYAMLStr := string(addonYAML)
			if !strings.HasPrefix(addonYAMLStr, "---") {
				addonYAML = []byte(fmt.Sprintf("%s\n%s", "---", addonYAMLStr))
			}
			if err := validateUserAddonYAML(addonYAML); err != nil {
				return err
			}
			manifests = append(manifests, addonYAML...)
		} else {
			log.Warnf(ctx, "[addons] Unable to determine if %s is a file path or url, skipping", addon)
		}
	}
	log.Infof(ctx, "[addons] Deploying %s", UserAddonsIncludeResourceName)
	logrus.Debugf("[addons] Compiled addons yaml: %s", string(manifests))

	return c.doAddonDeploy(ctx, string(manifests), UserAddonsIncludeResourceName, false)
}

func validateUserAddonYAML(addon []byte) error {
	yamlContents := make(map[string]interface{})

	return yaml.Unmarshal(addon, &yamlContents)
}

func isFilePath(addonPath string) bool {
	if _, err := os.Stat(addonPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func getAddonFromURL(yamlURL string) ([]byte, error) {
	resp, err := http.Get(yamlURL)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	addonYaml, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return addonYaml, nil

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
		addons.RBAC:                   c.Authorization.Mode,
	}
	kubeDNSYaml, err := addons.GetKubeDNSManifest(kubeDNSConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, kubeDNSYaml, KubeDNSAddonResourceName, false); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] KubeDNS deployed successfully..")
	return nil
}

func (c *Cluster) deployMetricServer(ctx context.Context) error {
	log.Infof(ctx, "[addons] Setting up Metrics Server")
	MetricsServerConfig := MetricsServerOptions{
		MetricsServerImage: c.SystemImages.MetricsServer,
		RBACConfig:         c.Authorization.Mode,
		Options:            c.Monitoring.Options,
	}
	kubeDNSYaml, err := addons.GetMetricsServerManifest(MetricsServerConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, kubeDNSYaml, MetricsServerAddonResourceName, false); err != nil {
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

func (c *Cluster) doAddonDeploy(ctx context.Context, addonYaml, resourceName string, isCritical bool) error {
	if c.UseKubectlDeploy {
		if err := c.deployWithKubectl(ctx, addonYaml); err != nil {
			return &addonError{fmt.Sprintf("%v", err), isCritical}
		}
	}

	addonUpdated, err := c.StoreAddonConfigMap(ctx, addonYaml, resourceName)
	if err != nil {
		return &addonError{fmt.Sprintf("Failed to save addon ConfigMap: %v", err), isCritical}
	}

	log.Infof(ctx, "[addons] Executing deploy job..")
	k8sClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return &addonError{fmt.Sprintf("%v", err), isCritical}
	}
	node, err := k8s.GetNode(k8sClient, c.ControlPlaneHosts[0].HostnameOverride)
	if err != nil {
		return &addonError{fmt.Sprintf("Failed to get Node [%s]: %v", c.ControlPlaneHosts[0].HostnameOverride, err), isCritical}
	}
	addonJob, err := addons.GetAddonsExecuteJob(resourceName, node.Name, c.Services.KubeAPI.Image)

	if err != nil {
		return &addonError{fmt.Sprintf("Failed to generate addon execute job: %v", err), isCritical}
	}

	if err = c.ApplySystemAddonExecuteJob(addonJob, addonUpdated); err != nil {
		return &addonError{fmt.Sprintf("%v", err), isCritical}
	}
	return nil
}

func (c *Cluster) doAddonDelete(ctx context.Context, resourceName string, isCritical bool) error {
	k8sClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return &addonError{fmt.Sprintf("%v", err), isCritical}
	}
	node, err := k8s.GetNode(k8sClient, c.ControlPlaneHosts[0].HostnameOverride)
	if err != nil {
		return &addonError{fmt.Sprintf("Failed to get Node [%s]: %v", c.ControlPlaneHosts[0].HostnameOverride, err), isCritical}
	}
	deleteJob, err := addons.GetAddonsDeleteJob(resourceName, node.Name, c.Services.KubeAPI.Image)
	if err != nil {
		return &addonError{fmt.Sprintf("Failed to generate addon delete job: %v", err), isCritical}
	}
	if err := k8s.ApplyK8sSystemJob(deleteJob, c.LocalKubeConfigPath, c.K8sWrapTransport, c.AddonJobTimeout*2, false); err != nil {
		return &addonError{fmt.Sprintf("%v", err), isCritical}
	}
	// At this point, the addon should be deleted. We need to clean up by deleting the deploy and delete jobs.
	tmpJobYaml, err := addons.GetAddonsExecuteJob(resourceName, node.Name, c.Services.KubeAPI.Image)
	if err != nil {
		return err
	}
	if err := k8s.DeleteK8sSystemJob(tmpJobYaml, k8sClient, c.AddonJobTimeout); err != nil {
		return err
	}

	if err := k8s.DeleteK8sSystemJob(deleteJob, k8sClient, c.AddonJobTimeout); err != nil {
		return err
	}

	return nil

}

func (c *Cluster) StoreAddonConfigMap(ctx context.Context, addonYaml string, addonName string) (bool, error) {
	log.Infof(ctx, "[addons] Saving addon ConfigMap to Kubernetes")
	updated := false
	kubeClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return updated, err
	}
	timeout := make(chan bool, 1)
	go func() {
		for {

			updated, err = k8s.UpdateConfigMap(kubeClient, []byte(addonYaml), addonName)
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
		return updated, nil
	case <-time.After(time.Second * UpdateStateTimeout):
		return updated, fmt.Errorf("[addons] Timeout waiting for kubernetes to be ready")
	}
}

func (c *Cluster) ApplySystemAddonExecuteJob(addonJob string, addonUpdated bool) error {
	if err := k8s.ApplyK8sSystemJob(addonJob, c.LocalKubeConfigPath, c.K8sWrapTransport, c.AddonJobTimeout, addonUpdated); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) deployIngress(ctx context.Context) error {
	if c.Ingress.Provider == "none" {
		addonJobExists, err := addons.AddonJobExists(IngressAddonJobName, c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			return nil
		}
		if addonJobExists {
			log.Infof(ctx, "[ingress] removing installed ingress controller")
			if err := c.doAddonDelete(ctx, IngressAddonResourceName, false); err != nil {
				return err
			}

			log.Infof(ctx, "[ingress] ingress controller removed successfully")
		} else {
			log.Infof(ctx, "[ingress] ingress controller is disabled, skipping ingress controller")
		}
		return nil
	}
	log.Infof(ctx, "[ingress] Setting up %s ingress controller", c.Ingress.Provider)
	ingressConfig := ingressOptions{
		RBACConfig:     c.Authorization.Mode,
		Options:        c.Ingress.Options,
		NodeSelector:   c.Ingress.NodeSelector,
		ExtraArgs:      c.Ingress.ExtraArgs,
		AlpineImage:    c.SystemImages.Alpine,
		IngressImage:   c.SystemImages.Ingress,
		IngressBackend: c.SystemImages.IngressBackend,
	}
	// Currently only deploying nginx ingress controller
	ingressYaml, err := addons.GetNginxIngressManifest(ingressConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, ingressYaml, IngressAddonResourceName, false); err != nil {
		return err
	}
	log.Infof(ctx, "[ingress] ingress controller %s is successfully deployed", c.Ingress.Provider)
	return nil
}
