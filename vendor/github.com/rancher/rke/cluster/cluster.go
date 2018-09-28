package cluster

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/rancher/rke/authz"
	"github.com/rancher/rke/cloudprovider"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

type Cluster struct {
	v3.RancherKubernetesEngineConfig `yaml:",inline"`
	ConfigPath                       string
	LocalKubeConfigPath              string
	EtcdHosts                        []*hosts.Host
	WorkerHosts                      []*hosts.Host
	ControlPlaneHosts                []*hosts.Host
	InactiveHosts                    []*hosts.Host
	EtcdReadyHosts                   []*hosts.Host
	KubeClient                       *kubernetes.Clientset
	KubernetesServiceIP              net.IP
	Certificates                     map[string]pki.CertificatePKI
	ClusterDomain                    string
	ClusterCIDR                      string
	ClusterDNSServer                 string
	DockerDialerFactory              hosts.DialerFactory
	LocalConnDialerFactory           hosts.DialerFactory
	PrivateRegistriesMap             map[string]v3.PrivateRegistry
	K8sWrapTransport                 k8s.WrapTransport
	UseKubectlDeploy                 bool
	UpdateWorkersOnly                bool
	CloudConfigFile                  string
}

const (
	X509AuthenticationProvider = "x509"
	StateConfigMapName         = "cluster-state"
	UpdateStateTimeout         = 30
	GetStateTimeout            = 30
	KubernetesClientTimeOut    = 30
	NoneAuthorizationMode      = "none"
	LocalNodeAddress           = "127.0.0.1"
	LocalNodeHostname          = "localhost"
	LocalNodeUser              = "root"
	CloudProvider              = "CloudProvider"
	ControlPlane               = "controlPlane"
	WorkerPlane                = "workerPlan"
	EtcdPlane                  = "etcd"
)

func (c *Cluster) DeployControlPlane(ctx context.Context) error {
	// Deploy Etcd Plane
	etcdNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build etcd node plan map
	for _, etcdHost := range c.EtcdHosts {
		etcdNodePlanMap[etcdHost.Address] = BuildRKEConfigNodePlan(ctx, c, etcdHost, etcdHost.DockerInfo)
	}

	if len(c.Services.Etcd.ExternalURLs) > 0 {
		log.Infof(ctx, "[etcd] External etcd connection string has been specified, skipping etcd plane")
	} else {
		etcdRollingSnapshot := services.EtcdSnapshot{
			Snapshot:  c.Services.Etcd.Snapshot,
			Creation:  c.Services.Etcd.Creation,
			Retention: c.Services.Etcd.Retention,
		}
		if err := services.RunEtcdPlane(ctx, c.EtcdHosts, etcdNodePlanMap, c.LocalConnDialerFactory, c.PrivateRegistriesMap, c.UpdateWorkersOnly, c.SystemImages.Alpine, etcdRollingSnapshot); err != nil {
			return fmt.Errorf("[etcd] Failed to bring up Etcd Plane: %v", err)
		}
	}

	// Deploy Control plane
	cpNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build cp node plan map
	for _, cpHost := range c.ControlPlaneHosts {
		cpNodePlanMap[cpHost.Address] = BuildRKEConfigNodePlan(ctx, c, cpHost, cpHost.DockerInfo)
	}
	if err := services.RunControlPlane(ctx, c.ControlPlaneHosts,
		c.LocalConnDialerFactory,
		c.PrivateRegistriesMap,
		cpNodePlanMap,
		c.UpdateWorkersOnly,
		c.SystemImages.Alpine,
		c.Certificates); err != nil {
		return fmt.Errorf("[controlPlane] Failed to bring up Control Plane: %v", err)
	}

	return nil
}

func (c *Cluster) DeployWorkerPlane(ctx context.Context) error {
	// Deploy Worker plane
	workerNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build cp node plan map
	allHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	for _, workerHost := range allHosts {
		workerNodePlanMap[workerHost.Address] = BuildRKEConfigNodePlan(ctx, c, workerHost, workerHost.DockerInfo)
	}
	if err := services.RunWorkerPlane(ctx, allHosts,
		c.LocalConnDialerFactory,
		c.PrivateRegistriesMap,
		workerNodePlanMap,
		c.Certificates,
		c.UpdateWorkersOnly,
		c.SystemImages.Alpine); err != nil {
		return fmt.Errorf("[workerPlane] Failed to bring up Worker Plane: %v", err)
	}
	return nil
}

func ParseConfig(clusterFile string) (*v3.RancherKubernetesEngineConfig, error) {
	logrus.Debugf("Parsing cluster file [%v]", clusterFile)
	var rkeConfig v3.RancherKubernetesEngineConfig
	if err := yaml.Unmarshal([]byte(clusterFile), &rkeConfig); err != nil {
		return nil, err
	}
	return &rkeConfig, nil
}

func ParseCluster(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	clusterFilePath, configDir string,
	dockerDialerFactory,
	localConnDialerFactory hosts.DialerFactory,
	k8sWrapTransport k8s.WrapTransport) (*Cluster, error) {
	var err error
	c := &Cluster{
		RancherKubernetesEngineConfig: *rkeConfig,
		ConfigPath:                    clusterFilePath,
		DockerDialerFactory:           dockerDialerFactory,
		LocalConnDialerFactory:        localConnDialerFactory,
		PrivateRegistriesMap:          make(map[string]v3.PrivateRegistry),
		K8sWrapTransport:              k8sWrapTransport,
	}
	// Setting cluster Defaults
	c.setClusterDefaults(ctx)

	if err := c.InvertIndexHosts(); err != nil {
		return nil, fmt.Errorf("Failed to classify hosts from config file: %v", err)
	}

	if err := c.ValidateCluster(); err != nil {
		return nil, fmt.Errorf("Failed to validate cluster: %v", err)
	}

	c.KubernetesServiceIP, err = pki.GetKubernetesServiceIP(c.Services.KubeAPI.ServiceClusterIPRange)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Kubernetes Service IP: %v", err)
	}
	c.ClusterDomain = c.Services.Kubelet.ClusterDomain
	c.ClusterCIDR = c.Services.KubeController.ClusterCIDR
	c.ClusterDNSServer = c.Services.Kubelet.ClusterDNSServer
	if len(c.ConfigPath) == 0 {
		c.ConfigPath = pki.ClusterConfig
	}
	c.LocalKubeConfigPath = pki.GetLocalKubeConfig(c.ConfigPath, configDir)

	for _, pr := range c.PrivateRegistries {
		if pr.URL == "" {
			pr.URL = docker.DockerRegistryURL
		}
		c.PrivateRegistriesMap[pr.URL] = pr
	}
	// Get Cloud Provider
	p, err := cloudprovider.InitCloudProvider(c.CloudProvider)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize cloud provider: %v", err)
	}
	if p != nil {
		c.CloudConfigFile, err = p.GenerateCloudConfigFile()
		if err != nil {
			return nil, fmt.Errorf("Failed to parse cloud config file: %v", err)
		}
		c.CloudProvider.Name = p.GetName()
		if c.CloudProvider.Name == "" {
			return nil, fmt.Errorf("Name of the cloud provider is not defined for custom provider")
		}
	}

	// Create k8s wrap transport for bastion host
	if len(c.BastionHost.Address) > 0 {
		var err error
		c.K8sWrapTransport, err = hosts.BastionHostWrapTransport(c.BastionHost)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func rebuildLocalAdminConfig(ctx context.Context, kubeCluster *Cluster) error {
	if len(kubeCluster.ControlPlaneHosts) == 0 {
		return nil
	}
	log.Infof(ctx, "[reconcile] Rebuilding and updating local kube config")
	var workingConfig, newConfig string
	currentKubeConfig := kubeCluster.Certificates[pki.KubeAdminCertName]
	caCrt := kubeCluster.Certificates[pki.CACertName].Certificate
	for _, cpHost := range kubeCluster.ControlPlaneHosts {
		if (currentKubeConfig == pki.CertificatePKI{}) {
			kubeCluster.Certificates = make(map[string]pki.CertificatePKI)
			newConfig = getLocalAdminConfigWithNewAddress(kubeCluster.LocalKubeConfigPath, cpHost.Address, kubeCluster.ClusterName)
		} else {
			kubeURL := fmt.Sprintf("https://%s:6443", cpHost.Address)
			caData := string(cert.EncodeCertPEM(caCrt))
			crtData := string(cert.EncodeCertPEM(currentKubeConfig.Certificate))
			keyData := string(cert.EncodePrivateKeyPEM(currentKubeConfig.Key))
			newConfig = pki.GetKubeConfigX509WithData(kubeURL, kubeCluster.ClusterName, pki.KubeAdminCertName, caData, crtData, keyData)
		}
		if err := pki.DeployAdminConfig(ctx, newConfig, kubeCluster.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to redeploy local admin config with new host")
		}
		workingConfig = newConfig
		if _, err := GetK8sVersion(kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err == nil {
			log.Infof(ctx, "[reconcile] host [%s] is active master on the cluster", cpHost.Address)
			break
		}
	}
	currentKubeConfig.Config = workingConfig
	kubeCluster.Certificates[pki.KubeAdminCertName] = currentKubeConfig
	return nil
}

func isLocalConfigWorking(ctx context.Context, localKubeConfigPath string, k8sWrapTransport k8s.WrapTransport) bool {
	if _, err := GetK8sVersion(localKubeConfigPath, k8sWrapTransport); err != nil {
		log.Infof(ctx, "[reconcile] Local config is not valid, rebuilding admin config")
		return false
	}
	return true
}

func getLocalConfigAddress(localConfigPath string) (string, error) {
	config, err := clientcmd.BuildConfigFromFlags("", localConfigPath)
	if err != nil {
		return "", err
	}
	splittedAdress := strings.Split(config.Host, ":")
	address := splittedAdress[1]
	return address[2:], nil
}

func getLocalAdminConfigWithNewAddress(localConfigPath, cpAddress string, clusterName string) string {
	config, _ := clientcmd.BuildConfigFromFlags("", localConfigPath)
	if config == nil {
		return ""
	}
	config.Host = fmt.Sprintf("https://%s:6443", cpAddress)
	return pki.GetKubeConfigX509WithData(
		"https://"+cpAddress+":6443",
		clusterName,
		pki.KubeAdminCertName,
		string(config.CAData),
		string(config.CertData),
		string(config.KeyData))
}

func ApplyAuthzResources(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, clusterFilePath, configDir string, k8sWrapTransport k8s.WrapTransport) error {
	// dialer factories are not needed here since we are not uses docker only k8s jobs
	kubeCluster, err := ParseCluster(ctx, &rkeConfig, clusterFilePath, configDir, nil, nil, k8sWrapTransport)
	if err != nil {
		return err
	}
	if len(kubeCluster.ControlPlaneHosts) == 0 {
		return nil
	}
	if err := authz.ApplyJobDeployerServiceAccount(ctx, kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err != nil {
		return fmt.Errorf("Failed to apply the ServiceAccount needed for job execution: %v", err)
	}
	if kubeCluster.Authorization.Mode == NoneAuthorizationMode {
		return nil
	}
	if kubeCluster.Authorization.Mode == services.RBACAuthorizationMode {
		if err := authz.ApplySystemNodeClusterRoleBinding(ctx, kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply the ClusterRoleBinding needed for node authorization: %v", err)
		}
	}
	if kubeCluster.Authorization.Mode == services.RBACAuthorizationMode && kubeCluster.Services.KubeAPI.PodSecurityPolicy {
		if err := authz.ApplyDefaultPodSecurityPolicy(ctx, kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy: %v", err)
		}
		if err := authz.ApplyDefaultPodSecurityPolicyRole(ctx, kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy ClusterRole and ClusterRoleBinding: %v", err)
		}
	}
	return nil
}

func (c *Cluster) deployAddons(ctx context.Context) error {
	if err := c.deployK8sAddOns(ctx); err != nil {
		return err
	}
	if err := c.deployUserAddOns(ctx); err != nil {
		if err, ok := err.(*addonError); ok && err.isCritical {
			return err
		}
		log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", UserAddonsIncludeResourceName, err)

	}
	return nil
}

func (c *Cluster) SyncLabelsAndTaints(ctx context.Context, currentCluster *Cluster) error {
	// Handle issue when deleting all controlplane nodes https://github.com/rancher/rancher/issues/15810
	if currentCluster != nil {
		cpToDelete := hosts.GetToDeleteHosts(currentCluster.ControlPlaneHosts, c.ControlPlaneHosts, c.InactiveHosts)
		if len(cpToDelete) == len(currentCluster.ControlPlaneHosts) {
			log.Infof(ctx, "[sync] Cleaning left control plane nodes from reconcilation")
			for _, toDeleteHost := range cpToDelete {
				if err := cleanControlNode(ctx, c, currentCluster, toDeleteHost); err != nil {
					return err
				}
			}
		}
	}

	if len(c.ControlPlaneHosts) > 0 {
		log.Infof(ctx, "[sync] Syncing nodes Labels and Taints")
		k8sClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
		}
		for _, host := range hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts) {
			if err := k8s.SetAddressesAnnotations(k8sClient, host.HostnameOverride, host.InternalAddress, host.Address); err != nil {
				return err
			}
			if err := k8s.SyncLabels(k8sClient, host.HostnameOverride, host.ToAddLabels, host.ToDelLabels); err != nil {
				return err
			}
			// Taints are not being added by user
			if err := k8s.SyncTaints(k8sClient, host.HostnameOverride, host.ToAddTaints, host.ToDelTaints); err != nil {
				return err
			}
		}
		log.Infof(ctx, "[sync] Successfully synced nodes Labels and Taints")
	}
	return nil
}

func (c *Cluster) PrePullK8sImages(ctx context.Context) error {
	log.Infof(ctx, "Pre-pulling kubernetes images")
	var errgrp errgroup.Group
	hosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	for _, host := range hosts {
		runHost := host
		errgrp.Go(func() error {
			return docker.UseLocalOrPull(ctx, runHost.DClient, runHost.Address, c.SystemImages.Kubernetes, "pre-deploy", c.PrivateRegistriesMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "Kubernetes images pulled successfully")
	return nil
}

func ConfigureCluster(
	ctx context.Context,
	rkeConfig v3.RancherKubernetesEngineConfig,
	crtBundle map[string]pki.CertificatePKI,
	clusterFilePath, configDir string,
	k8sWrapTransport k8s.WrapTransport,
	useKubectl bool) error {
	// dialer factories are not needed here since we are not uses docker only k8s jobs
	kubeCluster, err := ParseCluster(ctx, &rkeConfig, clusterFilePath, configDir, nil, nil, k8sWrapTransport)
	if err != nil {
		return err
	}
	kubeCluster.UseKubectlDeploy = useKubectl
	if len(kubeCluster.ControlPlaneHosts) > 0 {
		kubeCluster.Certificates = crtBundle
		if err := kubeCluster.deployNetworkPlugin(ctx); err != nil {
			if err, ok := err.(*addonError); ok && err.isCritical {
				return err
			}
			log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", NetworkPluginResourceName, err)
		}
		if err := kubeCluster.deployAddons(ctx); err != nil {
			return err
		}
	}
	return nil
}
