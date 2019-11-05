package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/rancher/rke/authz"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/metadata"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

type Cluster struct {
	AuthnStrategies                  map[string]bool
	ConfigPath                       string
	ConfigDir                        string
	CloudConfigFile                  string
	ControlPlaneHosts                []*hosts.Host
	Certificates                     map[string]pki.CertificatePKI
	CertificateDir                   string
	ClusterDomain                    string
	ClusterCIDR                      string
	ClusterDNSServer                 string
	DinD                             bool
	DockerDialerFactory              hosts.DialerFactory
	EtcdHosts                        []*hosts.Host
	EtcdReadyHosts                   []*hosts.Host
	ForceDeployCerts                 bool
	InactiveHosts                    []*hosts.Host
	K8sWrapTransport                 transport.WrapperFunc
	KubeClient                       *kubernetes.Clientset
	KubernetesServiceIP              net.IP
	LocalKubeConfigPath              string
	LocalConnDialerFactory           hosts.DialerFactory
	PrivateRegistriesMap             map[string]v3.PrivateRegistry
	StateFilePath                    string
	UpdateWorkersOnly                bool
	UseKubectlDeploy                 bool
	v3.RancherKubernetesEngineConfig `yaml:",inline"`
	WorkerHosts                      []*hosts.Host
	EncryptionConfig                 encryptionConfig
}

type encryptionConfig struct {
	RewriteSecrets         bool
	RotateKey              bool
	EncryptionProviderFile string
}

const (
	AuthnX509Provider      = "x509"
	AuthnWebhookProvider   = "webhook"
	StateConfigMapName     = "cluster-state"
	FullStateConfigMapName = "full-cluster-state"
	UpdateStateTimeout     = 30
	GetStateTimeout        = 30
	SyncWorkers            = 10
	NoneAuthorizationMode  = "none"
	LocalNodeAddress       = "127.0.0.1"
	LocalNodeHostname      = "localhost"
	LocalNodeUser          = "root"
	CloudProvider          = "CloudProvider"
	ControlPlane           = "controlPlane"
	KubeAppLabel           = "k8s-app"
	AppLabel               = "app"
	NameLabel              = "name"

	WorkerThreads = util.WorkerThreads

	serviceAccountTokenFileParam = "service-account-key-file"

	SystemNamespace = "kube-system"
)

func (c *Cluster) DeployControlPlane(ctx context.Context, svcOptionData map[string]*v3.KubernetesServicesOptions) error {
	// Deploy Etcd Plane
	etcdNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build etcd node plan map
	for _, etcdHost := range c.EtcdHosts {
		etcdNodePlanMap[etcdHost.Address] = BuildRKEConfigNodePlan(ctx, c, etcdHost, etcdHost.DockerInfo, svcOptionData)
	}

	if len(c.Services.Etcd.ExternalURLs) > 0 {
		log.Infof(ctx, "[etcd] External etcd connection string has been specified, skipping etcd plane")
	} else {
		if err := services.RunEtcdPlane(ctx, c.EtcdHosts, etcdNodePlanMap, c.LocalConnDialerFactory, c.PrivateRegistriesMap, c.UpdateWorkersOnly, c.SystemImages.Alpine, c.Services.Etcd, c.Certificates); err != nil {
			return fmt.Errorf("[etcd] Failed to bring up Etcd Plane: %v", err)
		}
	}

	// Deploy Control plane
	cpNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build cp node plan map
	for _, cpHost := range c.ControlPlaneHosts {
		cpNodePlanMap[cpHost.Address] = BuildRKEConfigNodePlan(ctx, c, cpHost, cpHost.DockerInfo, svcOptionData)
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

func (c *Cluster) DeployWorkerPlane(ctx context.Context, svcOptionData map[string]*v3.KubernetesServicesOptions) error {
	// Deploy Worker plane
	workerNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build cp node plan map
	allHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	for _, workerHost := range allHosts {
		workerNodePlanMap[workerHost.Address] = BuildRKEConfigNodePlan(ctx, c, workerHost, workerHost.DockerInfo, svcOptionData)
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

func parseAuditLogConfig(clusterFile string, rkeConfig *v3.RancherKubernetesEngineConfig) error {
	if rkeConfig.Services.KubeAPI.AuditLog != nil &&
		rkeConfig.Services.KubeAPI.AuditLog.Enabled &&
		rkeConfig.Services.KubeAPI.AuditLog.Configuration.Policy == nil {
		return nil
	}
	logrus.Debugf("audit log policy found in cluster.yml")
	var r map[string]interface{}
	err := ghodssyaml.Unmarshal([]byte(clusterFile), &r)
	if err != nil {
		return fmt.Errorf("error unmarshalling: %v", err)
	}
	if r["services"] == nil {
		return nil
	}
	services := r["services"].(map[string]interface{})
	if services["kube-api"] == nil {
		return nil
	}
	kubeapi := services["kube-api"].(map[string]interface{})
	if kubeapi["audit_log"] == nil {
		return nil
	}
	auditlog := kubeapi["audit_log"].(map[string]interface{})
	if auditlog["configuration"] == nil {
		return nil
	}
	alc := auditlog["configuration"].(map[string]interface{})
	if alc["policy"] == nil {
		return nil
	}
	policyBytes, err := json.Marshal(alc["policy"])
	if err != nil {
		return fmt.Errorf("error marshalling audit policy: %v", err)
	}
	scheme := runtime.NewScheme()
	err = auditv1.AddToScheme(scheme)
	if err != nil {
		return fmt.Errorf("error adding to scheme: %v", err)
	}
	codecs := serializer.NewCodecFactory(scheme)
	p := auditv1.Policy{}
	err = runtime.DecodeInto(codecs.UniversalDecoder(), policyBytes, &p)
	if err != nil || p.Kind != "Policy" {
		return fmt.Errorf("error decoding audit policy: %v", err)
	}
	rkeConfig.Services.KubeAPI.AuditLog.Configuration.Policy = &p
	return err
}

func parseAdmissionConfig(clusterFile string, rkeConfig *v3.RancherKubernetesEngineConfig) error {
	if rkeConfig.Services.KubeAPI.AdmissionConfiguration == nil {
		return nil
	}
	logrus.Debugf("admission configuration found in cluster.yml")
	var r map[string]interface{}
	err := ghodssyaml.Unmarshal([]byte(clusterFile), &r)
	if err != nil {
		return fmt.Errorf("error unmarshalling: %v", err)
	}
	if r["services"] == nil {
		return nil
	}
	services := r["services"].(map[string]interface{})
	if services["kube-api"] == nil {
		return nil
	}
	kubeapi := services["kube-api"].(map[string]interface{})
	if kubeapi["admission_configuration"] == nil {
		return nil
	}
	data, err := json.Marshal(kubeapi["admission_configuration"])
	if err != nil {
		return fmt.Errorf("error marshalling admission configuration: %v", err)
	}
	scheme := runtime.NewScheme()
	err = apiserverv1alpha1.AddToScheme(scheme)
	if err != nil {
		return fmt.Errorf("error adding to scheme: %v", err)
	}
	err = scheme.SetVersionPriority(apiserverv1alpha1.SchemeGroupVersion)
	if err != nil {
		return fmt.Errorf("error setting version priority: %v", err)
	}
	codecs := serializer.NewCodecFactory(scheme)
	decoder := codecs.UniversalDecoder(apiserverv1alpha1.SchemeGroupVersion)
	decodedObj, err := runtime.Decode(decoder, data)
	if err != nil {
		return fmt.Errorf("error decoding data: %v", err)
	}
	decodedConfig, ok := decodedObj.(*apiserverv1alpha1.AdmissionConfiguration)
	if !ok {
		return fmt.Errorf("unexpected type: %T", decodedObj)
	}
	rkeConfig.Services.KubeAPI.AdmissionConfiguration = decodedConfig
	return nil
}

func ParseConfig(clusterFile string) (*v3.RancherKubernetesEngineConfig, error) {
	logrus.Debugf("Parsing cluster file [%v]", clusterFile)
	var rkeConfig v3.RancherKubernetesEngineConfig

	// the customConfig is mapped to a k8s type, which doesn't unmarshal well because it has a
	// nested struct and no yaml tags. Therefor, we have to re-parse it again and assign it correctly.
	// this only affects rke cli. Since rkeConfig is passed from rancher directly in the rancher use case.
	clusterFile, secretConfig, err := resolveCustomEncryptionConfig(clusterFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(clusterFile), &rkeConfig); err != nil {
		return nil, err
	}

	if isEncryptionEnabled(&rkeConfig) && secretConfig != nil {
		rkeConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig = secretConfig
	}
	if err := parseAdmissionConfig(clusterFile, &rkeConfig); err != nil {
		return &rkeConfig, fmt.Errorf("error parsing admission config: %v", err)
	}
	if err := parseAuditLogConfig(clusterFile, &rkeConfig); err != nil {
		return &rkeConfig, fmt.Errorf("error parsing audit log config: %v", err)
	}
	return &rkeConfig, nil
}

func InitClusterObject(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, flags ExternalFlags, encryptConfig string) (*Cluster, error) {
	// basic cluster object from rkeConfig
	var err error
	c := &Cluster{
		AuthnStrategies:               make(map[string]bool),
		RancherKubernetesEngineConfig: *rkeConfig,
		ConfigPath:                    flags.ClusterFilePath,
		ConfigDir:                     flags.ConfigDir,
		DinD:                          flags.DinD,
		CertificateDir:                flags.CertificateDir,
		StateFilePath:                 GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir),
		PrivateRegistriesMap:          make(map[string]v3.PrivateRegistry),
		EncryptionConfig: encryptionConfig{
			EncryptionProviderFile: encryptConfig,
		},
	}
	if metadata.K8sVersionToRKESystemImages == nil {
		metadata.InitMetadata(ctx)
	}
	if len(c.ConfigPath) == 0 {
		c.ConfigPath = pki.ClusterConfig
	}
	// set kube_config, state file, and certificate dir
	c.LocalKubeConfigPath = pki.GetLocalKubeConfig(c.ConfigPath, c.ConfigDir)
	c.StateFilePath = GetStateFilePath(c.ConfigPath, c.ConfigDir)
	if len(c.CertificateDir) == 0 {
		c.CertificateDir = GetCertificateDirPath(c.ConfigPath, c.ConfigDir)
	}
	// We don't manage custom configuration, if it's there we just use it.
	if isEncryptionCustomConfig(rkeConfig) {
		if c.EncryptionConfig.EncryptionProviderFile, err = c.readEncryptionCustomConfig(); err != nil {
			return nil, err
		}
	} else if isEncryptionEnabled(rkeConfig) && c.EncryptionConfig.EncryptionProviderFile == "" {
		if c.EncryptionConfig.EncryptionProviderFile, err = c.getEncryptionProviderFile(); err != nil {
			return nil, err
		}
	}

	// Setting cluster Defaults
	err = c.setClusterDefaults(ctx, flags)
	if err != nil {
		return nil, err
	}
	// extract cluster network configuration
	c.setNetworkOptions()

	// Register cloud provider
	if err := c.setCloudProvider(); err != nil {
		return nil, fmt.Errorf("Failed to register cloud provider: %v", err)
	}
	// set hosts groups
	if err := c.InvertIndexHosts(); err != nil {
		return nil, fmt.Errorf("Failed to classify hosts from config file: %v", err)
	}
	// validate cluster configuration
	if err := c.ValidateCluster(ctx); err != nil {
		return nil, fmt.Errorf("Failed to validate cluster: %v", err)
	}
	return c, nil
}

func (c *Cluster) setNetworkOptions() error {
	var err error
	c.KubernetesServiceIP, err = pki.GetKubernetesServiceIP(c.Services.KubeAPI.ServiceClusterIPRange)
	if err != nil {
		return fmt.Errorf("Failed to get Kubernetes Service IP: %v", err)
	}
	c.ClusterDomain = c.Services.Kubelet.ClusterDomain
	c.ClusterCIDR = c.Services.KubeController.ClusterCIDR
	c.ClusterDNSServer = c.Services.Kubelet.ClusterDNSServer
	return nil
}

func (c *Cluster) SetupDialers(ctx context.Context, dailersOptions hosts.DialersOptions) error {
	c.DockerDialerFactory = dailersOptions.DockerDialerFactory
	c.LocalConnDialerFactory = dailersOptions.LocalConnDialerFactory
	c.K8sWrapTransport = dailersOptions.K8sWrapTransport
	// Create k8s wrap transport for bastion host
	if len(c.BastionHost.Address) > 0 {
		var err error
		c.K8sWrapTransport, err = hosts.BastionHostWrapTransport(c.BastionHost)
		if err != nil {
			return err
		}
	}
	return nil
}

func RebuildKubeconfig(ctx context.Context, kubeCluster *Cluster) error {
	return rebuildLocalAdminConfig(ctx, kubeCluster)
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
			return fmt.Errorf("Failed to redeploy local admin config with new host: %v", err)
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

func isLocalConfigWorking(ctx context.Context, localKubeConfigPath string, k8sWrapTransport transport.WrapperFunc) bool {
	if _, err := GetK8sVersion(localKubeConfigPath, k8sWrapTransport); err != nil {
		log.Infof(ctx, "[reconcile] Local config is not valid (error: %v), rebuilding admin config", err)
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
	if config == nil || config.BearerToken != "" {
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

func ApplyAuthzResources(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, flags ExternalFlags, dailersOptions hosts.DialersOptions) error {
	// dialer factories are not needed here since we are not uses docker only k8s jobs
	kubeCluster, err := InitClusterObject(ctx, &rkeConfig, flags, "")
	if err != nil {
		return err
	}
	if err := kubeCluster.SetupDialers(ctx, dailersOptions); err != nil {
		return err
	}
	if len(kubeCluster.ControlPlaneHosts) == 0 {
		return nil
	}
	// Print proxy environment variables as we are directly contacting the cluster
	util.PrintProxyEnvVars()
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
		if err := authz.ApplyKubeAPIClusterRole(ctx, kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply the ClusterRole and Binding needed for node kubeapi proxy: %v", err)
		}
	}
	if kubeCluster.Authorization.Mode == services.RBACAuthorizationMode && kubeCluster.Services.KubeAPI.PodSecurityPolicy {
		if err := authz.ApplyDefaultPodSecurityPolicy(ctx, kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy: %v", err)
		}
		if err := authz.ApplyDefaultPodSecurityPolicyRole(ctx, kubeCluster.LocalKubeConfigPath, SystemNamespace, kubeCluster.K8sWrapTransport); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy ClusterRole and ClusterRoleBinding: %v", err)
		}
	}
	return nil
}

func (c *Cluster) deployAddons(ctx context.Context, data map[string]interface{}) error {
	if err := c.deployK8sAddOns(ctx, data); err != nil {
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
		cpToDelete := hosts.GetToDeleteHosts(currentCluster.ControlPlaneHosts, c.ControlPlaneHosts, c.InactiveHosts, false)
		if len(cpToDelete) == len(currentCluster.ControlPlaneHosts) {
			log.Infof(ctx, "[sync] Cleaning left control plane nodes from reconciliation")
			for _, toDeleteHost := range cpToDelete {
				if err := cleanControlNode(ctx, c, currentCluster, toDeleteHost); err != nil {
					return err
				}
			}
		}
	}

	// sync node taints. Add or remove taints from hosts
	syncTaints(ctx, currentCluster, c)

	if len(c.ControlPlaneHosts) > 0 {
		log.Infof(ctx, "[sync] Syncing nodes Labels and Taints")
		k8sClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
		}
		hostList := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
		var errgrp errgroup.Group
		hostQueue := make(chan *hosts.Host, len(hostList))
		for _, host := range hostList {
			hostQueue <- host
		}
		close(hostQueue)

		for i := 0; i < SyncWorkers; i++ {
			w := i
			errgrp.Go(func() error {
				var errs []error
				for host := range hostQueue {
					logrus.Debugf("worker [%d] starting sync for node [%s]", w, host.HostnameOverride)
					if err := setNodeAnnotationsLabelsTaints(k8sClient, host); err != nil {
						errs = append(errs, err)
					}
				}
				if len(errs) > 0 {
					return fmt.Errorf("%v", errs)
				}
				return nil
			})
		}
		if err := errgrp.Wait(); err != nil {
			return err
		}
		log.Infof(ctx, "[sync] Successfully synced nodes Labels and Taints")
	}
	return nil
}

func setNodeAnnotationsLabelsTaints(k8sClient *kubernetes.Clientset, host *hosts.Host) error {
	node := &v1.Node{}
	var err error
	for retries := 0; retries <= 5; retries++ {
		node, err = k8s.GetNode(k8sClient, host.HostnameOverride)
		if err != nil {
			logrus.Debugf("[hosts] Can't find node by name [%s], retrying..", host.HostnameOverride)
			time.Sleep(2 * time.Second)
			continue
		}

		oldNode := node.DeepCopy()
		k8s.SetNodeAddressesAnnotations(node, host.InternalAddress, host.Address)
		k8s.SyncNodeLabels(node, host.ToAddLabels, host.ToDelLabels)
		k8s.SyncNodeTaints(node, host.ToAddTaints, host.ToDelTaints)

		if reflect.DeepEqual(oldNode, node) {
			logrus.Debugf("skipping syncing labels for node [%s]", node.Name)
			return nil
		}
		_, err = k8sClient.CoreV1().Nodes().Update(node)
		if err != nil {
			logrus.Debugf("Error syncing labels for node [%s]: %v", node.Name, err)
			time.Sleep(5 * time.Second)
			continue
		}
		return nil
	}
	return err
}

func (c *Cluster) PrePullK8sImages(ctx context.Context) error {
	log.Infof(ctx, "Pre-pulling kubernetes images")
	var errgrp errgroup.Group
	hostList := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	hostsQueue := util.GetObjectQueue(hostList)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				err := docker.UseLocalOrPull(ctx, runHost.DClient, runHost.Address, c.SystemImages.Kubernetes, "pre-deploy", c.PrivateRegistriesMap)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
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
	flags ExternalFlags,
	dailersOptions hosts.DialersOptions,
	data map[string]interface{},
	useKubectl bool) error {
	// dialer factories are not needed here since we are not uses docker only k8s jobs
	kubeCluster, err := InitClusterObject(ctx, &rkeConfig, flags, "")
	if err != nil {
		return err
	}
	if err := kubeCluster.SetupDialers(ctx, dailersOptions); err != nil {
		return err
	}
	kubeCluster.UseKubectlDeploy = useKubectl
	if len(kubeCluster.ControlPlaneHosts) > 0 {
		kubeCluster.Certificates = crtBundle
		if err := kubeCluster.deployNetworkPlugin(ctx, data); err != nil {
			if err, ok := err.(*addonError); ok && err.isCritical {
				return err
			}
			log.Warnf(ctx, "Failed to deploy addon execute job [%s]: %v", NetworkPluginResourceName, err)
		}
		if err := kubeCluster.deployAddons(ctx, data); err != nil {
			return err
		}
	}
	return nil
}

func RestartClusterPods(ctx context.Context, kubeCluster *Cluster) error {
	log.Infof(ctx, "Restarting network, ingress, and metrics pods")
	// this will remove the pods created by RKE and let the controller creates them again
	kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}
	labelsList := []string{
		fmt.Sprintf("%s=%s", KubeAppLabel, FlannelNetworkPlugin),
		fmt.Sprintf("%s=%s", KubeAppLabel, CanalNetworkPlugin),
		fmt.Sprintf("%s=%s", NameLabel, WeaveNetworkAppName),
		fmt.Sprintf("%s=%s", AppLabel, NginxIngressAddonAppName),
		fmt.Sprintf("%s=%s", KubeAppLabel, DefaultMonitoringProvider),
		fmt.Sprintf("%s=%s", KubeAppLabel, KubeDNSAddonAppName),
		fmt.Sprintf("%s=%s", KubeAppLabel, KubeDNSAutoscalerAppName),
		fmt.Sprintf("%s=%s", KubeAppLabel, CoreDNSAutoscalerAppName),
		fmt.Sprintf("%s=%s", AppLabel, KubeAPIAuthAppName),
		fmt.Sprintf("%s=%s", AppLabel, CattleClusterAgentAppName),
	}
	for _, calicoLabel := range CalicoNetworkLabels {
		labelsList = append(labelsList, fmt.Sprintf("%s=%s", KubeAppLabel, calicoLabel))
	}
	var errgrp errgroup.Group
	labelQueue := util.GetObjectQueue(labelsList)
	for w := 0; w < services.WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for label := range labelQueue {
				runLabel := label.(string)
				// list pods to be deleted
				pods, err := k8s.ListPodsByLabel(kubeClient, runLabel)
				if err != nil {
					errList = append(errList, err)
				}
				// delete pods
				err = k8s.DeletePods(kubeClient, pods)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) GetHostInfoMap() map[string]types.Info {
	hostsInfoMap := make(map[string]types.Info)
	allHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	for _, host := range allHosts {
		hostsInfoMap[host.Address] = host.DockerInfo
	}
	return hostsInfoMap
}

func IsLegacyKubeAPI(ctx context.Context, kubeCluster *Cluster) (bool, error) {
	log.Infof(ctx, "[controlplane] Check if rotating a legacy cluster")
	for _, host := range kubeCluster.ControlPlaneHosts {
		kubeAPIInspect, err := docker.InspectContainer(ctx, host.DClient, host.Address, services.KubeAPIContainerName)
		if err != nil {
			return false, err
		}
		for _, arg := range kubeAPIInspect.Args {
			if strings.Contains(arg, serviceAccountTokenFileParam) &&
				strings.Contains(arg, pki.GetKeyPath(pki.KubeAPICertName)) {
				return true, nil
			}
		}
	}
	return false, nil
}
