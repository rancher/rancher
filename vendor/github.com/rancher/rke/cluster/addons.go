package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rancher/rke/addons"
	"github.com/rancher/rke/authz"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/templates"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/kdm"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	UserAddonResourceName         = "rke-user-addon"
	IngressAddonResourceName      = "rke-ingress-controller"
	UserAddonsIncludeResourceName = "rke-user-includes-addons"

	IngressAddonJobName            = "rke-ingress-controller-deploy-job"
	MetricsServerAddonJobName      = "rke-metrics-addon-deploy-job"
	MetricsServerAddonResourceName = "rke-metrics-addon"
	NginxIngressAddonAppName       = "ingress-nginx"
	KubeDNSAddonAppName            = "kube-dns"
	KubeDNSAutoscalerAppName       = "kube-dns-autoscaler"
	CoreDNSAutoscalerAppName       = "coredns-autoscaler"
	KubeAPIAuthAppName             = "kube-api-auth"
	CattleClusterAgentAppName      = "cattle-cluster-agent"

	CoreDNSProvider = "coredns"
	KubeDNSProvider = "kube-dns"
	Nodelocal       = "nodelocal"
)

var DNSProviders = []string{KubeDNSProvider, CoreDNSProvider}

type ingressOptions struct {
	RBACConfig        string
	Options           map[string]string
	NodeSelector      map[string]string
	ExtraArgs         map[string]string
	ExtraEnvs         []v3.ExtraEnv
	ExtraVolumes      []v3.ExtraVolume
	ExtraVolumeMounts []v3.ExtraVolumeMount
	DNSPolicy         string
	AlpineImage       string
	IngressImage      string
	IngressBackend    string
	UpdateStrategy    *appsv1.DaemonSetUpdateStrategy
}

type MetricsServerOptions struct {
	RBACConfig         string
	Options            map[string]string
	NodeSelector       map[string]string
	MetricsServerImage string
	Version            string
	UpdateStrategy     *appsv1.DeploymentStrategy
	Replicas           *int32
}

type CoreDNSOptions struct {
	RBACConfig             string
	CoreDNSImage           string
	CoreDNSAutoScalerImage string
	ClusterDomain          string
	ClusterDNSServer       string
	ReverseCIDRs           []string
	UpstreamNameservers    []string
	NodeSelector           map[string]string
	UpdateStrategy         *appsv1.DeploymentStrategy
	LinearAutoscalerParams string
}

type KubeDNSOptions struct {
	RBACConfig             string
	KubeDNSImage           string
	DNSMasqImage           string
	KubeDNSAutoScalerImage string
	KubeDNSSidecarImage    string
	ClusterDomain          string
	ClusterDNSServer       string
	ReverseCIDRs           []string
	UpstreamNameservers    []string
	StubDomains            map[string][]string
	NodeSelector           map[string]string
	UpdateStrategy         *appsv1.DeploymentStrategy
	LinearAutoscalerParams string
}

type NodelocalOptions struct {
	RBACConfig       string
	NodelocalImage   string
	ClusterDomain    string
	ClusterDNSServer string
	IPAddress        string
	NodeSelector     map[string]string
	UpdateStrategy   *appsv1.DaemonSetUpdateStrategy
}

type addonError struct {
	err        string
	isCritical bool
}

func (e *addonError) Error() string {
	return e.err
}

func getAddonResourceName(addon string) string {
	AddonResourceName := "rke-" + addon + "-addon"
	return AddonResourceName
}

func (c *Cluster) deployK8sAddOns(ctx context.Context, data map[string]interface{}) error {
	if err := c.deployDNS(ctx, data); err != nil {
		if err, ok := err.(*addonError); ok && err.isCritical {
			return err
		}
		log.Warnf(ctx, "Failed to deploy DNS addon execute job for provider %s: %v", c.DNS.Provider, err)

	}
	if err := c.deployMetricServer(ctx, data); err != nil {
		if err, ok := err.(*addonError); ok && err.isCritical {
			return err
		}
		log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", MetricsServerAddonResourceName, err)
	}
	if err := c.deployIngress(ctx, data); err != nil {
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
		log.Infof(ctx, "[addons] No included addon paths or urls")
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

			// make sure we properly separated manifests
			addonYAMLStr := string(addonYAML)

			formattedAddonYAML := formatAddonYAML(addonYAMLStr)

			addonYAML = []byte(formattedAddonYAML)
			logrus.Debugf("Formatted Yaml: %s", addonYAML)

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

			formattedAddonYAML := formatAddonYAML(addonYAMLStr)

			addonYAML = []byte(formattedAddonYAML)
			logrus.Debugf("Formatted Yaml: %s", addonYAML)

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

func formatAddonYAML(addonYAMLStr string) string {
	if !strings.HasPrefix(addonYAMLStr, "---") {
		logrus.Debug("Yaml does not start with dashes")
		addonYAMLStr = fmt.Sprintf("%s\n%s", "---", addonYAMLStr)
	}

	if !strings.HasSuffix(addonYAMLStr, "\n") {
		logrus.Debug("Yaml does not end with newline")
		addonYAMLStr = fmt.Sprintf("%s\n", addonYAMLStr)
	}
	return addonYAMLStr
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

func (c *Cluster) deployKubeDNS(ctx context.Context, data map[string]interface{}) error {
	log.Infof(ctx, "[addons] Setting up %s", c.DNS.Provider)
	KubeDNSConfig := KubeDNSOptions{
		KubeDNSImage:           c.SystemImages.KubeDNS,
		KubeDNSSidecarImage:    c.SystemImages.KubeDNSSidecar,
		KubeDNSAutoScalerImage: c.SystemImages.KubeDNSAutoscaler,
		DNSMasqImage:           c.SystemImages.DNSmasq,
		RBACConfig:             c.Authorization.Mode,
		ClusterDomain:          c.ClusterDomain,
		ClusterDNSServer:       c.ClusterDNSServer,
		UpstreamNameservers:    c.DNS.UpstreamNameservers,
		ReverseCIDRs:           c.DNS.ReverseCIDRs,
		StubDomains:            c.DNS.StubDomains,
		NodeSelector:           c.DNS.NodeSelector,
		UpdateStrategy: &appsv1.DeploymentStrategy{
			Type:          c.DNS.UpdateStrategy.Strategy,
			RollingUpdate: c.DNS.UpdateStrategy.RollingUpdate,
		},
	}
	linearModeBytes, err := json.Marshal(c.DNS.LinearAutoscalerParams)
	if err != nil {
		return err
	}
	KubeDNSConfig.LinearAutoscalerParams = string(linearModeBytes)
	tmplt, err := templates.GetVersionedTemplates(kdm.KubeDNS, data, c.Version)
	if err != nil {
		return err
	}
	kubeDNSYaml, err := templates.CompileTemplateFromMap(tmplt, KubeDNSConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, kubeDNSYaml, getAddonResourceName(c.DNS.Provider), false); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] %s deployed successfully", c.DNS.Provider)
	return nil
}

func (c *Cluster) deployCoreDNS(ctx context.Context, data map[string]interface{}) error {
	log.Infof(ctx, "[addons] Setting up %s", c.DNS.Provider)
	CoreDNSConfig := CoreDNSOptions{
		CoreDNSImage:           c.SystemImages.CoreDNS,
		CoreDNSAutoScalerImage: c.SystemImages.CoreDNSAutoscaler,
		RBACConfig:             c.Authorization.Mode,
		ClusterDomain:          c.ClusterDomain,
		ClusterDNSServer:       c.ClusterDNSServer,
		UpstreamNameservers:    c.DNS.UpstreamNameservers,
		ReverseCIDRs:           c.DNS.ReverseCIDRs,
		NodeSelector:           c.DNS.NodeSelector,
		UpdateStrategy: &appsv1.DeploymentStrategy{
			Type:          c.DNS.UpdateStrategy.Strategy,
			RollingUpdate: c.DNS.UpdateStrategy.RollingUpdate,
		},
	}
	linearModeBytes, err := json.Marshal(c.DNS.LinearAutoscalerParams)
	if err != nil {
		return err
	}
	CoreDNSConfig.LinearAutoscalerParams = string(linearModeBytes)
	tmplt, err := templates.GetVersionedTemplates(kdm.CoreDNS, data, c.Version)
	if err != nil {
		return err
	}
	coreDNSYaml, err := templates.CompileTemplateFromMap(tmplt, CoreDNSConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, coreDNSYaml, getAddonResourceName(c.DNS.Provider), false); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] CoreDNS deployed successfully")
	return nil
}

func (c *Cluster) deployMetricServer(ctx context.Context, data map[string]interface{}) error {
	if c.Monitoring.Provider == "none" {
		addonJobExists, err := addons.AddonJobExists(MetricsServerAddonJobName, c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			return nil
		}
		if addonJobExists {
			log.Infof(ctx, "[ingress] Removing installed metrics server")
			if err := c.doAddonDelete(ctx, MetricsServerAddonResourceName, false); err != nil {
				return err
			}

			log.Infof(ctx, "[ingress] Metrics server removed successfully")
		} else {
			log.Infof(ctx, "[ingress] Metrics Server is disabled, skipping Metrics server installation")
		}
		return nil
	}
	log.Infof(ctx, "[addons] Setting up Metrics Server")
	s := strings.Split(c.SystemImages.MetricsServer, ":")
	versionTag := s[len(s)-1]

	MetricsServerConfig := MetricsServerOptions{
		MetricsServerImage: c.SystemImages.MetricsServer,
		RBACConfig:         c.Authorization.Mode,
		Options:            c.Monitoring.Options,
		NodeSelector:       c.Monitoring.NodeSelector,
		Version:            util.GetTagMajorVersion(versionTag),
		UpdateStrategy: &appsv1.DeploymentStrategy{
			Type:          c.Monitoring.UpdateStrategy.Strategy,
			RollingUpdate: c.Monitoring.UpdateStrategy.RollingUpdate,
		},
		Replicas: c.Monitoring.Replicas,
	}
	tmplt, err := templates.GetVersionedTemplates(kdm.MetricsServer, data, c.Version)
	if err != nil {
		return err
	}
	metricsYaml, err := templates.CompileTemplateFromMap(tmplt, MetricsServerConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, metricsYaml, MetricsServerAddonResourceName, false); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] Metrics Server deployed successfully")
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

	log.Infof(ctx, "[addons] Executing deploy job %s", resourceName)
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
	log.Infof(ctx, "[addons] Saving ConfigMap for addon %s to Kubernetes", addonName)
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
				continue
			}
			log.Infof(ctx, "[addons] Successfully saved ConfigMap for addon %s to Kubernetes", addonName)
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

func (c *Cluster) deployIngress(ctx context.Context, data map[string]interface{}) error {
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
		RBACConfig:        c.Authorization.Mode,
		Options:           c.Ingress.Options,
		NodeSelector:      c.Ingress.NodeSelector,
		ExtraArgs:         c.Ingress.ExtraArgs,
		DNSPolicy:         c.Ingress.DNSPolicy,
		IngressImage:      c.SystemImages.Ingress,
		IngressBackend:    c.SystemImages.IngressBackend,
		ExtraEnvs:         c.Ingress.ExtraEnvs,
		ExtraVolumes:      c.Ingress.ExtraVolumes,
		ExtraVolumeMounts: c.Ingress.ExtraVolumeMounts,
		UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
			Type:          c.Ingress.UpdateStrategy.Strategy,
			RollingUpdate: c.Ingress.UpdateStrategy.RollingUpdate,
		},
	}
	// since nginx ingress controller 0.16.0, it can be run as non-root and doesn't require privileged anymore.
	// So we can use securityContext instead of setting privileges via initContainer.
	ingressSplits := strings.SplitN(c.SystemImages.Ingress, ":", 2)
	if len(ingressSplits) == 2 {
		version := strings.Split(ingressSplits[1], "-")[0]
		if version < "0.16.0" {
			ingressConfig.AlpineImage = c.SystemImages.Alpine
		}
	}
	tmplt, err := templates.GetVersionedTemplates(kdm.NginxIngress, data, c.Version)
	if err != nil {
		return err
	}
	// Currently only deploying nginx ingress controller
	ingressYaml, err := templates.CompileTemplateFromMap(tmplt, ingressConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, ingressYaml, IngressAddonResourceName, false); err != nil {
		return err
	}
	// ingress runs in it's own namespace, so it needs it's own role/rolebinding for PSP
	if c.Authorization.Mode == services.RBACAuthorizationMode && c.Services.KubeAPI.PodSecurityPolicy {
		if err := authz.ApplyDefaultPodSecurityPolicyRole(ctx, c.LocalKubeConfigPath, NginxIngressAddonAppName, c.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy ClusterRole and ClusterRoleBinding: %v", err)
		}
	}
	log.Infof(ctx, "[ingress] ingress controller %s deployed successfully", c.Ingress.Provider)
	return nil
}
func (c *Cluster) removeDNSProvider(ctx context.Context, dnsprovider string) error {
	AddonJobExists, err := addons.AddonJobExists(getAddonResourceName(dnsprovider)+"-deploy-job", c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return err
	}
	if AddonJobExists {
		log.Infof(ctx, "[dns] removing DNS provider %s", dnsprovider)
		if err := c.doAddonDelete(ctx, getAddonResourceName(dnsprovider), false); err != nil {
			return err
		}

		log.Infof(ctx, "[dns] DNS provider %s removed successfully", dnsprovider)
		return nil
	}
	return nil
}

func (c *Cluster) deployDNS(ctx context.Context, data map[string]interface{}) error {
	for _, dnsprovider := range DNSProviders {
		if strings.EqualFold(dnsprovider, c.DNS.Provider) {
			continue
		}
		if err := c.removeDNSProvider(ctx, dnsprovider); err != nil {
			return err
		}
	}
	switch DNSProvider := c.DNS.Provider; DNSProvider {
	case DefaultDNSProvider:
		if err := c.deployKubeDNS(ctx, data); err != nil {
			if err, ok := err.(*addonError); ok && err.isCritical {
				return err
			}
			log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", getAddonResourceName(c.DNS.Provider), err)
		}
		log.Infof(ctx, "[dns] DNS provider %s deployed successfully", c.DNS.Provider)
	case CoreDNSProvider:
		if err := c.deployCoreDNS(ctx, data); err != nil {
			if err, ok := err.(*addonError); ok && err.isCritical {
				return err
			}
			log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", getAddonResourceName(c.DNS.Provider), err)
		}
		log.Infof(ctx, "[dns] DNS provider %s deployed successfully", c.DNS.Provider)
	case "none":
		return nil
	default:
		log.Warnf(ctx, "[dns] No valid DNS provider configured: %s", c.DNS.Provider)
		return nil
	}
	// Check for nodelocal DNS
	if c.DNS.Nodelocal == nil {
		AddonJobExists, err := addons.AddonJobExists(getAddonResourceName(Nodelocal)+"-deploy-job", c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			return err
		}
		if AddonJobExists {
			log.Infof(ctx, "[dns] removing %s", Nodelocal)
			if err := c.doAddonDelete(ctx, getAddonResourceName(Nodelocal), false); err != nil {
				return err
			}

			log.Infof(ctx, "[dns] %s removed successfully", Nodelocal)
			return nil
		}
	}
	if c.DNS.Nodelocal != nil && c.DNS.Nodelocal.IPAddress != "" {
		if err := c.deployNodelocal(ctx, data); err != nil {
			if err, ok := err.(*addonError); ok && err.isCritical {
				return err
			}
			log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", getAddonResourceName(Nodelocal), err)
		}
		return nil
	}
	return nil
}

func (c *Cluster) deployNodelocal(ctx context.Context, data map[string]interface{}) error {
	log.Infof(ctx, "[dns] Setting up %s", Nodelocal)
	NodelocalConfig := NodelocalOptions{
		NodelocalImage:   c.SystemImages.Nodelocal,
		RBACConfig:       c.Authorization.Mode,
		ClusterDomain:    c.ClusterDomain,
		ClusterDNSServer: c.ClusterDNSServer,
		IPAddress:        c.DNS.Nodelocal.IPAddress,
		NodeSelector:     c.DNS.Nodelocal.NodeSelector,
	}
	if c.DNS.Nodelocal.UpdateStrategy != nil {
		NodelocalConfig.UpdateStrategy = &appsv1.DaemonSetUpdateStrategy{
			Type:          c.DNS.Nodelocal.UpdateStrategy.Strategy,
			RollingUpdate: c.DNS.Nodelocal.UpdateStrategy.RollingUpdate,
		}
	}
	tmplt, err := templates.GetVersionedTemplates(kdm.Nodelocal, data, c.Version)
	if err != nil {
		return err
	}
	nodelocalYaml, err := templates.CompileTemplateFromMap(tmplt, NodelocalConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, nodelocalYaml, getAddonResourceName(Nodelocal), false); err != nil {
		return err
	}
	log.Infof(ctx, "[dns] %s deployed successfully", Nodelocal)
	return nil
}
