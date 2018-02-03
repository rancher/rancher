package cluster

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/rancher/rke/authz"
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
	KubeClient                       *kubernetes.Clientset
	KubernetesServiceIP              net.IP
	Certificates                     map[string]pki.CertificatePKI
	ClusterDomain                    string
	ClusterCIDR                      string
	ClusterDNSServer                 string
	DockerDialerFactory              hosts.DialerFactory
	LocalConnDialerFactory           hosts.DialerFactory
	PrivateRegistriesMap             map[string]v3.PrivateRegistry
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
)

func (c *Cluster) DeployControlPlane(ctx context.Context) error {
	// Deploy Etcd Plane
	if err := services.RunEtcdPlane(ctx, c.EtcdHosts, c.Services.Etcd, c.LocalConnDialerFactory, c.PrivateRegistriesMap); err != nil {
		return fmt.Errorf("[etcd] Failed to bring up Etcd Plane: %v", err)
	}
	// Deploy Control plane
	if err := services.RunControlPlane(ctx, c.ControlPlaneHosts,
		c.EtcdHosts,
		c.Services,
		c.SystemImages.KubernetesServicesSidecar,
		c.Authorization.Mode,
		c.LocalConnDialerFactory,
		c.PrivateRegistriesMap); err != nil {
		return fmt.Errorf("[controlPlane] Failed to bring up Control Plane: %v", err)
	}
	// Apply Authz configuration after deploying controlplane
	if err := c.ApplyAuthzResources(ctx); err != nil {
		return fmt.Errorf("[auths] Failed to apply RBAC resources: %v", err)
	}
	return nil
}

func (c *Cluster) DeployWorkerPlane(ctx context.Context) error {
	// Deploy Worker Plane
	if err := services.RunWorkerPlane(ctx, c.ControlPlaneHosts,
		c.WorkerHosts,
		c.EtcdHosts,
		c.Services,
		c.SystemImages.NginxProxy,
		c.SystemImages.KubernetesServicesSidecar,
		c.LocalConnDialerFactory,
		c.PrivateRegistriesMap); err != nil {
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
	localConnDialerFactory hosts.DialerFactory) (*Cluster, error) {
	var err error
	c := &Cluster{
		RancherKubernetesEngineConfig: *rkeConfig,
		ConfigPath:                    clusterFilePath,
		DockerDialerFactory:           dockerDialerFactory,
		LocalConnDialerFactory:        localConnDialerFactory,
		PrivateRegistriesMap:          make(map[string]v3.PrivateRegistry),
	}
	// Setting cluster Defaults
	c.setClusterDefaults(ctx)

	if err := c.InvertIndexHosts(); err != nil {
		return nil, fmt.Errorf("Failed to classify hosts from config file: %v", err)
	}

	if err := c.ValidateCluster(); err != nil {
		return nil, fmt.Errorf("Failed to validate cluster: %v", err)
	}

	c.KubernetesServiceIP, err = services.GetKubernetesServiceIP(c.Services.KubeAPI.ServiceClusterIPRange)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Kubernetes Service IP: %v", err)
	}
	c.ClusterDomain = c.Services.Kubelet.ClusterDomain
	c.ClusterCIDR = c.Services.KubeController.ClusterCIDR
	c.ClusterDNSServer = c.Services.Kubelet.ClusterDNSServer
	if len(c.ConfigPath) == 0 {
		c.ConfigPath = DefaultClusterConfig
	}
	c.LocalKubeConfigPath = GetLocalKubeConfig(c.ConfigPath, configDir)

	for _, pr := range c.PrivateRegistries {
		if pr.URL == "" {
			pr.URL = docker.DockerRegistryURL
		}
		c.PrivateRegistriesMap[pr.URL] = pr
	}

	return c, nil
}

func GetLocalKubeConfig(configPath, configDir string) string {
	baseDir := filepath.Dir(configPath)
	if len(configDir) > 0 {
		baseDir = filepath.Dir(configDir)
	}
	fileName := filepath.Base(configPath)
	baseDir += "/"
	return fmt.Sprintf("%s%s%s", baseDir, pki.KubeAdminConfigPrefix, fileName)
}

func rebuildLocalAdminConfig(ctx context.Context, kubeCluster *Cluster) error {
	log.Infof(ctx, "[reconcile] Rebuilding and updating local kube config")
	var workingConfig, newConfig string
	currentKubeConfig := kubeCluster.Certificates[pki.KubeAdminCertName]
	caCrt := kubeCluster.Certificates[pki.CACertName].Certificate
	for _, cpHost := range kubeCluster.ControlPlaneHosts {
		if (currentKubeConfig == pki.CertificatePKI{}) {
			kubeCluster.Certificates = make(map[string]pki.CertificatePKI)
			newConfig = getLocalAdminConfigWithNewAddress(kubeCluster.LocalKubeConfigPath, cpHost.Address)
		} else {
			kubeURL := fmt.Sprintf("https://%s:6443", cpHost.Address)
			caData := string(cert.EncodeCertPEM(caCrt))
			crtData := string(cert.EncodeCertPEM(currentKubeConfig.Certificate))
			keyData := string(cert.EncodePrivateKeyPEM(currentKubeConfig.Key))
			newConfig = pki.GetKubeConfigX509WithData(kubeURL, pki.KubeAdminCertName, caData, crtData, keyData)
		}
		if err := pki.DeployAdminConfig(ctx, newConfig, kubeCluster.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to redeploy local admin config with new host")
		}
		workingConfig = newConfig
		if _, err := GetK8sVersion(kubeCluster.LocalKubeConfigPath); err == nil {
			log.Infof(ctx, "[reconcile] host [%s] is active master on the cluster", cpHost.Address)
			break
		}
	}
	currentKubeConfig.Config = workingConfig
	kubeCluster.Certificates[pki.KubeAdminCertName] = currentKubeConfig
	return nil
}

func isLocalConfigWorking(ctx context.Context, localKubeConfigPath string) bool {
	if _, err := GetK8sVersion(localKubeConfigPath); err != nil {
		log.Infof(ctx, "[reconcile] Local config is not vaild, rebuilding admin config")
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

func getLocalAdminConfigWithNewAddress(localConfigPath, cpAddress string) string {
	config, _ := clientcmd.BuildConfigFromFlags("", localConfigPath)
	config.Host = fmt.Sprintf("https://%s:6443", cpAddress)
	return pki.GetKubeConfigX509WithData(
		"https://"+cpAddress+":6443",
		pki.KubeAdminCertName,
		string(config.CAData),
		string(config.CertData),
		string(config.KeyData))
}

func (c *Cluster) ApplyAuthzResources(ctx context.Context) error {
	if err := authz.ApplyJobDeployerServiceAccount(ctx, c.LocalKubeConfigPath); err != nil {
		return fmt.Errorf("Failed to apply the ServiceAccount needed for job execution: %v", err)
	}
	if c.Authorization.Mode == NoneAuthorizationMode {
		return nil
	}
	if c.Authorization.Mode == services.RBACAuthorizationMode {
		if err := authz.ApplySystemNodeClusterRoleBinding(ctx, c.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to apply the ClusterRoleBinding needed for node authorization: %v", err)
		}
	}
	if c.Authorization.Mode == services.RBACAuthorizationMode && c.Services.KubeAPI.PodSecurityPolicy {
		if err := authz.ApplyDefaultPodSecurityPolicy(ctx, c.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy: %v", err)
		}
		if err := authz.ApplyDefaultPodSecurityPolicyRole(ctx, c.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy ClusterRole and ClusterRoleBinding: %v", err)
		}
	}
	return nil
}

func (c *Cluster) getUniqueHostList() []*hosts.Host {
	hostList := []*hosts.Host{}
	hostList = append(hostList, c.EtcdHosts...)
	hostList = append(hostList, c.ControlPlaneHosts...)
	hostList = append(hostList, c.WorkerHosts...)
	// little trick to get a unique host list
	uniqHostMap := make(map[*hosts.Host]bool)
	for _, host := range hostList {
		uniqHostMap[host] = true
	}
	uniqHostList := []*hosts.Host{}
	for host := range uniqHostMap {
		uniqHostList = append(uniqHostList, host)
	}
	return uniqHostList
}

func (c *Cluster) DeployAddons(ctx context.Context) error {
	if err := c.DeployK8sAddOns(ctx); err != nil {
		return err
	}
	return c.DeployUserAddOns(ctx)
}

func (c *Cluster) SyncLabelsAndTaints(ctx context.Context) error {
	log.Infof(ctx, "[sync] Syncing nodes Labels and Taints")
	k8sClient, err := k8s.NewClient(c.LocalKubeConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}
	for _, host := range c.getUniqueHostList() {
		if err := k8s.SyncLabels(k8sClient, host.HostnameOverride, host.ToAddLabels, host.ToDelLabels); err != nil {
			return err
		}
		// Taints are not being added by user
		if err := k8s.SyncTaints(k8sClient, host.HostnameOverride, host.ToAddTaints, host.ToDelTaints); err != nil {
			return err
		}
	}
	log.Infof(ctx, "[sync] Successfully synced nodes Labels and Taints")
	return nil
}

func (c *Cluster) PrePullK8sImages(ctx context.Context) error {
	log.Infof(ctx, "Pre-pulling kubernetes images")
	var errgrp errgroup.Group
	hosts := c.getUniqueHostList()
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
