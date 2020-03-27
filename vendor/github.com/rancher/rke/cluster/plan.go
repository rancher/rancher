package cluster

import (
	"context"
	"crypto/md5"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/metadata"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	ClusterCIDREnv        = "RKE_CLUSTER_CIDR"
	ClusterServiceCIDREnv = "RKE_CLUSTER_SERVICE_CIDR"
	ClusterDNSServerEnv   = "RKE_CLUSTER_DNS_SERVER"
	ClusterDomainEnv      = "RKE_CLUSTER_DOMAIN"

	NodeAddressEnv         = "RKE_NODE_ADDRESS"
	NodeInternalAddressEnv = "RKE_NODE_INTERNAL_ADDRESS"
	NodeNameOverrideEnv    = "RKE_NODE_NAME_OVERRIDE"

	NetworkConfigurationEnv = "RKE_NETWORK_CONFIGURATION"

	EtcdPathPrefix       = "/registry"
	CloudConfigSumEnv    = "RKE_CLOUD_CONFIG_CHECKSUM"
	CloudProviderNameEnv = "RKE_CLOUD_PROVIDER_NAME"

	DefaultToolsEntrypoint        = "/opt/rke-tools/entrypoint.sh"
	DefaultToolsEntrypointVersion = "0.1.13"
	LegacyToolsEntrypoint         = "/opt/rke/entrypoint.sh"

	KubeletDockerConfigEnv     = "RKE_KUBELET_DOCKER_CONFIG"
	KubeletDockerConfigFileEnv = "RKE_KUBELET_DOCKER_FILE"
	KubeletDockerConfigPath    = "/var/lib/kubelet/config.json"

	// MaxEtcdOldEnvVersion The versions are maxed out for minor versions because -rancher1 suffix will cause semver to think its older, example: v1.15.0 > v1.15.0-rancher1
	MaxEtcdOldEnvVersion = "v3.2.99"
	MaxK8s115Version     = "v1.15"

	EncryptionProviderConfigArgument = "encryption-provider-config"
)

var admissionControlOptionNames = []string{"enable-admission-plugins", "admission-control"}

func GetServiceOptionData(data map[string]interface{}) map[string]*v3.KubernetesServicesOptions {
	svcOptionsData := map[string]*v3.KubernetesServicesOptions{}
	k8sServiceOptions, _ := data["k8s-service-options"].(*v3.KubernetesServicesOptions)
	if k8sServiceOptions != nil {
		svcOptionsData["k8s-service-options"] = k8sServiceOptions
	}
	k8sWServiceOptions, _ := data["k8s-windows-service-options"].(*v3.KubernetesServicesOptions)
	if k8sWServiceOptions != nil {
		svcOptionsData["k8s-windows-service-options"] = k8sWServiceOptions
	}
	return svcOptionsData
}

func GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, hostsInfoMap map[string]types.Info, data map[string]interface{}) (v3.RKEPlan, error) {
	clusterPlan := v3.RKEPlan{}
	myCluster, err := InitClusterObject(ctx, rkeConfig, ExternalFlags{}, "")
	if err != nil {
		return clusterPlan, err
	}
	// rkeConfig.Nodes are already unique. But they don't have role flags. So I will use the parsed cluster.Hosts to make use of the role flags.
	uniqHosts := hosts.GetUniqueHostList(myCluster.EtcdHosts, myCluster.ControlPlaneHosts, myCluster.WorkerHosts)
	svcOptionData := GetServiceOptionData(data)

	for _, host := range uniqHosts {
		host.DockerInfo = hostsInfoMap[host.Address]
		svcOptions, err := myCluster.GetKubernetesServicesOptions(host.DockerInfo.OSType, svcOptionData)
		if err != nil {
			return clusterPlan, err
		}
		clusterPlan.Nodes = append(clusterPlan.Nodes, BuildRKEConfigNodePlan(ctx, myCluster, host, hostsInfoMap[host.Address], svcOptions))
	}
	return clusterPlan, nil
}

func BuildRKEConfigNodePlan(ctx context.Context, myCluster *Cluster, host *hosts.Host, hostDockerInfo types.Info, svcOptions v3.KubernetesServicesOptions) v3.RKEConfigNodePlan {
	prefixPath := hosts.GetPrefixPath(hostDockerInfo.OperatingSystem, myCluster.PrefixPath)
	processes := map[string]v3.Process{}
	portChecks := []v3.PortCheck{}
	// Everybody gets a sidecar and a kubelet..
	processes[services.SidekickContainerName] = myCluster.BuildSidecarProcess(host, prefixPath)
	processes[services.KubeletContainerName] = myCluster.BuildKubeletProcess(host, prefixPath, svcOptions)
	processes[services.KubeproxyContainerName] = myCluster.BuildKubeProxyProcess(host, prefixPath, svcOptions)

	portChecks = append(portChecks, BuildPortChecksFromPortList(host, WorkerPortList, ProtocolTCP)...)
	// Do we need an nginxProxy for this one ?
	if !host.IsControl {
		processes[services.NginxProxyContainerName] = myCluster.BuildProxyProcess(host, prefixPath)
	}
	if host.IsControl {
		processes[services.KubeAPIContainerName] = myCluster.BuildKubeAPIProcess(host, prefixPath, svcOptions)
		processes[services.KubeControllerContainerName] = myCluster.BuildKubeControllerProcess(host, prefixPath, svcOptions)
		processes[services.SchedulerContainerName] = myCluster.BuildSchedulerProcess(host, prefixPath, svcOptions)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, ControlPlanePortList, ProtocolTCP)...)
	}
	if host.IsEtcd {
		processes[services.EtcdContainerName] = myCluster.BuildEtcdProcess(host, myCluster.EtcdReadyHosts, prefixPath, svcOptions)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, EtcdPortList, ProtocolTCP)...)
	}
	files := []v3.File{
		v3.File{
			Name:     cloudConfigFileName,
			Contents: b64.StdEncoding.EncodeToString([]byte(myCluster.CloudConfigFile)),
		},
	}
	if myCluster.IsEncryptionEnabled() {
		files = append(files, v3.File{
			Name:     EncryptionProviderFilePath,
			Contents: b64.StdEncoding.EncodeToString([]byte(myCluster.EncryptionConfig.EncryptionProviderFile)),
		})
	}
	return v3.RKEConfigNodePlan{
		Address:    host.Address,
		Processes:  osLimitationFilter(hostDockerInfo.OSType, processes),
		PortChecks: portChecks,
		Files:      files,
		Annotations: map[string]string{
			k8s.ExternalAddressAnnotation: host.Address,
			k8s.InternalAddressAnnotation: host.InternalAddress,
		},
		Labels: host.ToAddLabels,
	}
}

func osLimitationFilter(osType string, processes map[string]v3.Process) map[string]v3.Process {
	if osType != "windows" {
		return processes
	}

	// windows limitations
	for name, process := range processes {
		// doesn't support host network on windows
		if process.NetworkMode == "host" {
			process.NetworkMode = ""
		}

		// doesn't support PID on windows
		if process.PidMode != "" {
			process.PidMode = ""
		}

		// doesn't support privileged mode on windows
		if process.Privileged {
			process.Privileged = false
		}

		// doesn't execute health check
		process.HealthCheck = v3.HealthCheck{}

		processes[name] = process
	}

	return processes
}

func (c *Cluster) BuildKubeAPIProcess(host *hosts.Host, prefixPath string, serviceOptions v3.KubernetesServicesOptions) v3.Process {
	// check if external etcd is used
	etcdConnectionString := services.GetEtcdConnString(c.EtcdHosts, host.InternalAddress)
	etcdPathPrefix := EtcdPathPrefix
	etcdClientCert := pki.GetCertPath(pki.KubeNodeCertName)
	etcdClientKey := pki.GetKeyPath(pki.KubeNodeCertName)
	etcdCAClientCert := pki.GetCertPath(pki.CACertName)

	if len(c.Services.Etcd.ExternalURLs) > 0 {
		etcdConnectionString = strings.Join(c.Services.Etcd.ExternalURLs, ",")
		etcdPathPrefix = c.Services.Etcd.Path
		etcdClientCert = pki.GetCertPath(pki.EtcdClientCertName)
		etcdClientKey = pki.GetKeyPath(pki.EtcdClientCertName)
		etcdCAClientCert = pki.GetCertPath(pki.EtcdClientCACertName)
	}

	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-apiserver",
	}

	CommandArgs := map[string]string{
		"client-ca-file":               pki.GetCertPath(pki.CACertName),
		"cloud-provider":               c.CloudProvider.Name,
		"etcd-cafile":                  etcdCAClientCert,
		"etcd-certfile":                etcdClientCert,
		"etcd-keyfile":                 etcdClientKey,
		"etcd-prefix":                  etcdPathPrefix,
		"etcd-servers":                 etcdConnectionString,
		"kubelet-client-certificate":   pki.GetCertPath(pki.KubeAPICertName),
		"kubelet-client-key":           pki.GetKeyPath(pki.KubeAPICertName),
		"proxy-client-cert-file":       pki.GetCertPath(pki.APIProxyClientCertName),
		"proxy-client-key-file":        pki.GetKeyPath(pki.APIProxyClientCertName),
		"requestheader-allowed-names":  pki.APIProxyClientCertName,
		"requestheader-client-ca-file": pki.GetCertPath(pki.RequestHeaderCACertName),
		"service-account-key-file":     pki.GetKeyPath(pki.ServiceAccountTokenKeyName),
		"service-cluster-ip-range":     c.Services.KubeAPI.ServiceClusterIPRange,
		"service-node-port-range":      c.Services.KubeAPI.ServiceNodePortRange,
		"tls-cert-file":                pki.GetCertPath(pki.KubeAPICertName),
		"tls-private-key-file":         pki.GetKeyPath(pki.KubeAPICertName),
	}
	if len(c.CloudProvider.Name) > 0 {
		CommandArgs["cloud-config"] = cloudConfigFileName
	}
	if c.Authentication.Webhook != nil {
		CommandArgs["authentication-token-webhook-config-file"] = authnWebhookFileName
		CommandArgs["authentication-token-webhook-cache-ttl"] = c.Authentication.Webhook.CacheTimeout
	}
	if len(c.CloudProvider.Name) > 0 {
		c.Services.KubeAPI.ExtraEnv = append(
			c.Services.KubeAPI.ExtraEnv,
			fmt.Sprintf("%s=%s", CloudConfigSumEnv, getCloudConfigChecksum(c.CloudConfigFile)))
	}
	if c.EncryptionConfig.EncryptionProviderFile != "" {
		CommandArgs[EncryptionProviderConfigArgument] = EncryptionProviderFilePath
	}

	if c.IsKubeletGenerateServingCertificateEnabled() {
		CommandArgs["kubelet-certificate-authority"] = pki.GetCertPath(pki.CACertName)
	}

	if serviceOptions.KubeAPI != nil {
		for k, v := range serviceOptions.KubeAPI {
			// if the value is empty, we remove that option
			if len(v) == 0 {
				delete(CommandArgs, k)
				continue
			}
			CommandArgs[k] = v
		}
	}
	// check api server count for k8s v1.8
	if util.GetTagMajorVersion(c.Version) == "v1.8" {
		CommandArgs["apiserver-count"] = strconv.Itoa(len(c.ControlPlaneHosts))
	}

	if c.Authorization.Mode == services.RBACAuthorizationMode {
		CommandArgs["authorization-mode"] = "Node,RBAC"
	}

	if len(host.InternalAddress) > 0 && net.ParseIP(host.InternalAddress) != nil {
		CommandArgs["advertise-address"] = host.InternalAddress
	}

	admissionControlOptionName := ""
	for _, optionName := range admissionControlOptionNames {
		if _, ok := CommandArgs[optionName]; ok {
			admissionControlOptionName = optionName
			break
		}
	}

	if c.Services.KubeAPI.PodSecurityPolicy {
		CommandArgs["runtime-config"] = "policy/v1beta1/podsecuritypolicy=true"
		CommandArgs[admissionControlOptionName] = CommandArgs[admissionControlOptionName] + ",PodSecurityPolicy"
	}

	if c.Services.KubeAPI.AlwaysPullImages {
		CommandArgs[admissionControlOptionName] = CommandArgs[admissionControlOptionName] + ",AlwaysPullImages"
	}

	if c.Services.KubeAPI.EventRateLimit != nil && c.Services.KubeAPI.EventRateLimit.Enabled {
		CommandArgs[KubeAPIArgAdmissionControlConfigFile] = DefaultKubeAPIArgAdmissionControlConfigFileValue
		CommandArgs[admissionControlOptionName] = CommandArgs[admissionControlOptionName] + ",EventRateLimit"
	}

	if c.Services.KubeAPI.AuditLog != nil {
		if alc := c.Services.KubeAPI.AuditLog.Configuration; alc != nil {
			CommandArgs[KubeAPIArgAuditLogPath] = alc.Path
			CommandArgs[KubeAPIArgAuditLogMaxAge] = strconv.Itoa(alc.MaxAge)
			CommandArgs[KubeAPIArgAuditLogMaxBackup] = strconv.Itoa(alc.MaxBackup)
			CommandArgs[KubeAPIArgAuditLogMaxSize] = strconv.Itoa(alc.MaxSize)
			CommandArgs[KubeAPIArgAuditLogFormat] = alc.Format
			CommandArgs[KubeAPIArgAuditPolicyFile] = DefaultKubeAPIArgAuditPolicyFileValue
		}
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
	}
	if c.Services.KubeAPI.AuditLog != nil && c.Services.KubeAPI.AuditLog.Enabled {
		Binds = append(Binds, fmt.Sprintf("%s:/var/log/kube-audit:z", path.Join(prefixPath, "/var/log/kube-audit")))
	}

	// Override args if they exist, add additional args
	for arg, value := range c.Services.KubeAPI.ExtraArgs {
		if _, ok := c.Services.KubeAPI.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}

	Binds = append(Binds, c.Services.KubeAPI.ExtraBinds...)

	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(true, services.KubeAPIPort),
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.KubeAPI.Image, c.PrivateRegistriesMap)

	return v3.Process{
		Name:                    services.KubeAPIContainerName,
		Command:                 Command,
		VolumesFrom:             VolumesFrom,
		Binds:                   getUniqStringList(Binds),
		Env:                     getUniqStringList(c.Services.KubeAPI.ExtraEnv),
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.KubeAPI.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.KubeAPIContainerName,
		},
	}
}

func (c *Cluster) BuildKubeControllerProcess(host *hosts.Host, prefixPath string, serviceOptions v3.KubernetesServicesOptions) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-controller-manager",
	}

	CommandArgs := map[string]string{
		"cloud-provider":                   c.CloudProvider.Name,
		"cluster-cidr":                     c.ClusterCIDR,
		"kubeconfig":                       pki.GetConfigPath(pki.KubeControllerCertName),
		"root-ca-file":                     pki.GetCertPath(pki.CACertName),
		"service-account-private-key-file": pki.GetKeyPath(pki.ServiceAccountTokenKeyName),
		"service-cluster-ip-range":         c.Services.KubeController.ServiceClusterIPRange,
	}
	// Best security practice is to listen on localhost, but DinD uses private container network instead of Host.
	if c.DinD {
		CommandArgs["address"] = "0.0.0.0"
	}
	if len(c.CloudProvider.Name) > 0 {
		CommandArgs["cloud-config"] = cloudConfigFileName
	}
	if len(c.CloudProvider.Name) > 0 {
		c.Services.KubeController.ExtraEnv = append(
			c.Services.KubeController.ExtraEnv,
			fmt.Sprintf("%s=%s", CloudConfigSumEnv, getCloudConfigChecksum(c.CloudConfigFile)))
	}

	if serviceOptions.KubeController != nil {
		for k, v := range serviceOptions.KubeController {
			// if the value is empty, we remove that option
			if len(v) == 0 {
				delete(CommandArgs, k)
				continue
			}
			CommandArgs[k] = v
		}
	}

	args := []string{}
	if c.Authorization.Mode == services.RBACAuthorizationMode {
		args = append(args, "--use-service-account-credentials=true")
	}
	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
	}

	for arg, value := range c.Services.KubeController.ExtraArgs {
		if _, ok := c.Services.KubeController.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}

	Binds = append(Binds, c.Services.KubeController.ExtraBinds...)

	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.KubeControllerPort),
	}

	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.KubeController.Image, c.PrivateRegistriesMap)
	return v3.Process{
		Name:                    services.KubeControllerContainerName,
		Command:                 Command,
		Args:                    args,
		VolumesFrom:             VolumesFrom,
		Binds:                   getUniqStringList(Binds),
		Env:                     getUniqStringList(c.Services.KubeController.ExtraEnv),
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.KubeController.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.KubeControllerContainerName,
		},
	}
}

func (c *Cluster) BuildKubeletProcess(host *hosts.Host, prefixPath string, serviceOptions v3.KubernetesServicesOptions) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kubelet",
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Command = []string{
			"pwsh", "-NoLogo", "-NonInteractive", "-File", "c:/usr/bin/entrypoint.ps1",
			"kubelet",
		}
	}

	CommandArgs := map[string]string{
		"client-ca-file":            pki.GetCertPath(pki.CACertName),
		"cloud-provider":            c.CloudProvider.Name,
		"cluster-dns":               c.ClusterDNSServer,
		"cluster-domain":            c.ClusterDomain,
		"fail-swap-on":              strconv.FormatBool(c.Services.Kubelet.FailSwapOn),
		"hostname-override":         host.HostnameOverride,
		"kubeconfig":                pki.GetConfigPath(pki.KubeNodeCertName),
		"pod-infra-container-image": c.Services.Kubelet.InfraContainerImage,
		"root-dir":                  path.Join(prefixPath, "/var/lib/kubelet"),
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		CommandArgs["kubeconfig"] = path.Join(prefixPath, pki.GetConfigPath(pki.KubeNodeCertName))
		CommandArgs["client-ca-file"] = path.Join(prefixPath, pki.GetCertPath(pki.CACertName))
		// this's a stopgap, we could drop this after https://github.com/kubernetes/kubernetes/pull/75618 merged
		CommandArgs["pod-infra-container-image"] = c.SystemImages.WindowsPodInfraContainer
	}

	if c.DinD {
		CommandArgs["healthz-bind-address"] = "0.0.0.0"
	}

	if host.IsControl && !host.IsWorker {
		CommandArgs["register-with-taints"] = unschedulableControlTaint
	}
	if host.Address != host.InternalAddress {
		CommandArgs["node-ip"] = host.InternalAddress
	}
	if len(c.CloudProvider.Name) > 0 {
		CommandArgs["cloud-config"] = cloudConfigFileName
		if host.DockerInfo.OSType == "windows" { // compatible with Windows
			CommandArgs["cloud-config"] = path.Join(prefixPath, cloudConfigFileName)
		}
	}
	if c.IsKubeletGenerateServingCertificateEnabled() {
		CommandArgs["tls-cert-file"] = pki.GetCertPath(pki.GetCrtNameForHost(host, pki.KubeletCertName))
		CommandArgs["tls-private-key-file"] = pki.GetCertPath(fmt.Sprintf("%s-key", pki.GetCrtNameForHost(host, pki.KubeletCertName)))
	}

	if len(c.CloudProvider.Name) > 0 {
		c.Services.Kubelet.ExtraEnv = append(
			c.Services.Kubelet.ExtraEnv,
			fmt.Sprintf("%s=%s", CloudConfigSumEnv, getCloudConfigChecksum(c.CloudConfigFile)))
	}
	if len(c.PrivateRegistriesMap) > 0 {
		kubeletDockerConfig, _ := docker.GetKubeletDockerConfig(c.PrivateRegistriesMap)
		c.Services.Kubelet.ExtraEnv = append(
			c.Services.Kubelet.ExtraEnv,
			fmt.Sprintf("%s=%s", KubeletDockerConfigEnv,
				b64.StdEncoding.EncodeToString([]byte(kubeletDockerConfig))))

		c.Services.Kubelet.ExtraEnv = append(
			c.Services.Kubelet.ExtraEnv,
			fmt.Sprintf("%s=%s", KubeletDockerConfigFileEnv, path.Join(prefixPath, KubeletDockerConfigPath)))
	}

	if serviceOptions.Kubelet != nil {
		for k, v := range serviceOptions.Kubelet {
			// if the value is empty, we remove that option
			if len(v) == 0 {
				delete(CommandArgs, k)
				continue
			}

			// if the value is '', we set that option to empty string,
			// e.g.: there's not cgroup on windows, we need to empty `enforce-node-allocatable` option
			if v == "''" {
				CommandArgs[k] = ""
				continue
			}

			// if the value has [PREFIX_PATH] prefix, we need to replace it with `prefixPath`,
			// e.g.: windows allows to use other drivers than `c:`
			if strings.HasPrefix(v, "[PREFIX_PATH]") {
				CommandArgs[k] = path.Join(prefixPath, strings.Replace(v, "[PREFIX_PATH]", "", -1))
				continue
			}

			CommandArgs[k] = v
		}
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
		"/etc/cni:/etc/cni:rw,z",
		"/opt/cni:/opt/cni:rw,z",
		fmt.Sprintf("%s:/var/lib/cni:z", path.Join(prefixPath, "/var/lib/cni")),
		"/var/lib/calico:/var/lib/calico:z",
		"/etc/resolv.conf:/etc/resolv.conf",
		"/sys:/sys:rprivate",
		host.DockerInfo.DockerRootDir + ":" + host.DockerInfo.DockerRootDir + ":rw,rslave,z",
		fmt.Sprintf("%s:%s:shared,z", path.Join(prefixPath, "/var/lib/kubelet"), path.Join(prefixPath, "/var/lib/kubelet")),
		"/var/lib/rancher:/var/lib/rancher:shared,z",
		"/var/run:/var/run:rw,rprivate",
		"/run:/run:rprivate",
		fmt.Sprintf("%s:/etc/ceph", path.Join(prefixPath, "/etc/ceph")),
		"/dev:/host/dev:rprivate",
		"/var/log/containers:/var/log/containers:z",
		"/var/log/pods:/var/log/pods:z",
		"/usr:/host/usr:ro",
		"/etc:/host/etc:ro",
	}
	// Special case to simplify using flex volumes
	if path.Join(prefixPath, "/var/lib/kubelet") != "/var/lib/kubelet" {
		Binds = append(Binds, "/var/lib/kubelet/volumeplugins:/var/lib/kubelet/volumeplugins:shared,z")
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Binds = []string{
			// put the execution binaries and cloud provider configuration to the host
			fmt.Sprintf("%s:c:/host/etc/kubernetes", path.Join(prefixPath, "/etc/kubernetes")),
			// put the flexvolume plugins or private registry docker configuration to the host
			fmt.Sprintf("%s:c:/host/var/lib/kubelet", path.Join(prefixPath, "/var/lib/kubelet")),
			// exchange resources with other components
			fmt.Sprintf("%s:c:/host/run", path.Join(prefixPath, "/run")),
		}
	}

	Env := c.Services.Kubelet.ExtraEnv
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Env = append(Env,
			fmt.Sprintf("%s=%s", ClusterCIDREnv, c.ClusterCIDR),
			fmt.Sprintf("%s=%s", ClusterDomainEnv, c.ClusterDomain),
			fmt.Sprintf("%s=%s", ClusterDNSServerEnv, c.ClusterDNSServer),
			fmt.Sprintf("%s=%s", ClusterServiceCIDREnv, c.Services.KubeController.ServiceClusterIPRange),
			fmt.Sprintf("%s=%s", NodeAddressEnv, host.Address),
			fmt.Sprintf("%s=%s", NodeInternalAddressEnv, host.InternalAddress),
			fmt.Sprintf("%s=%s", CloudProviderNameEnv, c.CloudProvider.Name),
		)
	}

	for arg, value := range c.Services.Kubelet.ExtraArgs {
		if _, ok := c.Services.Kubelet.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	// If nodelocal DNS is configured, set cluster-dns to local IP
	if c.DNS.Nodelocal != nil && c.DNS.Nodelocal.IPAddress != "" {
		CommandArgs["cluster-dns"] = c.DNS.Nodelocal.IPAddress
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}

	Binds = append(Binds, c.Services.Kubelet.ExtraBinds...)

	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.KubeletPort),
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.Kubelet.Image, c.PrivateRegistriesMap)

	return v3.Process{
		Name:                    services.KubeletContainerName,
		Command:                 Command,
		VolumesFrom:             VolumesFrom,
		Binds:                   getUniqStringList(Binds),
		Env:                     getUniqStringList(Env),
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Kubelet.Image,
		PidMode:                 "host",
		Privileged:              true,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.KubeletContainerName,
		},
	}
}

func (c *Cluster) BuildKubeProxyProcess(host *hosts.Host, prefixPath string, serviceOptions v3.KubernetesServicesOptions) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-proxy",
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Command = []string{
			"pwsh", "-NoLogo", "-NonInteractive", "-File", "c:/usr/bin/entrypoint.ps1",
			"kube-proxy",
		}
	}

	CommandArgs := map[string]string{
		"cluster-cidr":      c.ClusterCIDR,
		"hostname-override": host.HostnameOverride,
		"kubeconfig":        pki.GetConfigPath(pki.KubeProxyCertName),
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		CommandArgs["kubeconfig"] = path.Join(prefixPath, pki.GetConfigPath(pki.KubeProxyCertName))
	}

	if serviceOptions.Kubeproxy != nil {
		for k, v := range serviceOptions.Kubeproxy {
			// if the value is empty, we remove that option
			if len(v) == 0 {
				delete(CommandArgs, k)
				continue
			}
			CommandArgs[k] = v
		}
	}
	// If cloudprovider is set (not empty), set the bind address because the node will not be able to retrieve it's IP address in case cloud provider changes the node object name (i.e. AWS and Openstack)
	if c.CloudProvider.Name != "" {
		if host.InternalAddress != "" && host.Address != host.InternalAddress {
			CommandArgs["bind-address"] = host.InternalAddress
		} else {
			CommandArgs["bind-address"] = host.Address
		}
	}

	// Best security practice is to listen on localhost, but DinD uses private container network instead of Host.
	if c.DinD {
		CommandArgs["healthz-bind-address"] = "0.0.0.0"
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	//TODO: we should reevaluate if any of the bind mounts here should be using read-only mode
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
		"/run:/run",
		"/lib/modules:/lib/modules:z,ro",
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Binds = []string{
			// put the execution binaries to the host
			fmt.Sprintf("%s:c:/host/etc/kubernetes", path.Join(prefixPath, "/etc/kubernetes")),
			// exchange resources with other components
			fmt.Sprintf("%s:c:/host/run", path.Join(prefixPath, "/run")),
		}
	}

	Env := c.Services.Kubeproxy.ExtraEnv
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Env = append(Env,
			fmt.Sprintf("%s=%s", ClusterCIDREnv, c.ClusterCIDR),
			fmt.Sprintf("%s=%s", ClusterDomainEnv, c.ClusterDomain),
			fmt.Sprintf("%s=%s", ClusterDNSServerEnv, c.ClusterDNSServer),
			fmt.Sprintf("%s=%s", ClusterServiceCIDREnv, c.Services.KubeController.ServiceClusterIPRange),
			fmt.Sprintf("%s=%s", NodeAddressEnv, host.Address),
			fmt.Sprintf("%s=%s", NodeInternalAddressEnv, host.InternalAddress),
			fmt.Sprintf("%s=%s", CloudProviderNameEnv, c.CloudProvider.Name),
		)
	}

	for arg, value := range c.Services.Kubeproxy.ExtraArgs {
		if _, ok := c.Services.Kubeproxy.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}

	Binds = append(Binds, c.Services.Kubeproxy.ExtraBinds...)

	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.KubeproxyPort),
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.Kubeproxy.Image, c.PrivateRegistriesMap)
	return v3.Process{
		Name:                    services.KubeproxyContainerName,
		Command:                 Command,
		VolumesFrom:             VolumesFrom,
		Binds:                   getUniqStringList(Binds),
		Env:                     getUniqStringList(Env),
		NetworkMode:             "host",
		RestartPolicy:           "always",
		PidMode:                 "host",
		Privileged:              true,
		HealthCheck:             healthCheck,
		Image:                   c.Services.Kubeproxy.Image,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.KubeproxyContainerName,
		},
	}
}

func (c *Cluster) BuildProxyProcess(host *hosts.Host, prefixPath string) v3.Process {
	Command := []string{
		"nginx-proxy",
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Command = []string{
			"pwsh", "-NoLogo", "-NonInteractive", "-File", "c:/usr/bin/nginx-proxy.ps1",
		}
	}

	nginxProxyEnv := ""
	for i, host := range c.ControlPlaneHosts {
		nginxProxyEnv += fmt.Sprintf("%s", host.InternalAddress)
		if i < (len(c.ControlPlaneHosts) - 1) {
			nginxProxyEnv += ","
		}
	}
	Env := []string{fmt.Sprintf("%s=%s", services.NginxProxyEnvName, nginxProxyEnv)}

	VolumesFrom := []string{}
	if host.DockerInfo.OSType == "windows" { // compatible withe Windows
		VolumesFrom = []string{
			services.SidekickContainerName,
		}
	}

	Binds := []string{}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Binds = []string{
			// put the execution binaries and generate the configuration to the host
			fmt.Sprintf("%s:c:/host/etc/nginx", path.Join(prefixPath, "/etc/nginx")),
			// exchange resources with other components
			fmt.Sprintf("%s:c:/host/run", path.Join(prefixPath, "/run")),
		}
	}

	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.SystemImages.NginxProxy, c.PrivateRegistriesMap)
	return v3.Process{
		Name: services.NginxProxyContainerName,
		Env:  Env,
		// we do this to force container update when CP hosts change.
		Args:                    Env,
		Command:                 Command,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Binds:                   Binds,
		VolumesFrom:             VolumesFrom,
		HealthCheck:             v3.HealthCheck{},
		Image:                   c.SystemImages.NginxProxy,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.NginxProxyContainerName,
		},
	}
}

func (c *Cluster) BuildSchedulerProcess(host *hosts.Host, prefixPath string, serviceOptions v3.KubernetesServicesOptions) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-scheduler",
	}

	CommandArgs := map[string]string{
		"kubeconfig": pki.GetConfigPath(pki.KubeSchedulerCertName),
	}

	// Best security practice is to listen on localhost, but DinD uses private container network instead of Host.
	if c.DinD {
		CommandArgs["address"] = "0.0.0.0"
	}

	if serviceOptions.Scheduler != nil {
		for k, v := range serviceOptions.Scheduler {
			// if the value is empty, we remove that option
			if len(v) == 0 {
				delete(CommandArgs, k)
				continue
			}
			CommandArgs[k] = v
		}
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
	}

	for arg, value := range c.Services.Scheduler.ExtraArgs {
		if _, ok := c.Services.Scheduler.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}

	Binds = append(Binds, c.Services.Scheduler.ExtraBinds...)

	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.SchedulerPort),
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.Scheduler.Image, c.PrivateRegistriesMap)
	return v3.Process{
		Name:                    services.SchedulerContainerName,
		Command:                 Command,
		Binds:                   getUniqStringList(Binds),
		Env:                     c.Services.Scheduler.ExtraEnv,
		VolumesFrom:             VolumesFrom,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Scheduler.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.SchedulerContainerName,
		},
	}
}

func (c *Cluster) BuildSidecarProcess(host *hosts.Host, prefixPath string) v3.Process {
	Command := []string{
		"/bin/bash",
	}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		// windows docker doesn't support host network mode,
		// so we can't use the network components installed by addon like Linux.
		// we use sidecar container to maintain the network components
		Command = []string{
			"pwsh", "-NoLogo", "-NonInteractive", "-File", "c:/usr/bin/sidecar.ps1",
		}
	}

	Env := []string{}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Env = append(Env,
			fmt.Sprintf("%s=%s", ClusterCIDREnv, c.ClusterCIDR),
			fmt.Sprintf("%s=%s", ClusterDomainEnv, c.ClusterDomain),
			fmt.Sprintf("%s=%s", ClusterDNSServerEnv, c.ClusterDNSServer),
			fmt.Sprintf("%s=%s", ClusterServiceCIDREnv, c.Services.KubeController.ServiceClusterIPRange),
			fmt.Sprintf("%s=%s", NodeAddressEnv, host.Address),
			fmt.Sprintf("%s=%s", NodeInternalAddressEnv, host.InternalAddress),
			fmt.Sprintf("%s=%s", CloudProviderNameEnv, c.CloudProvider.Name),
			// sidekick needs the node name to drive the cni network management, e.g: flanneld
			fmt.Sprintf("%s=%s", NodeNameOverrideEnv, host.HostnameOverride),
			// sidekick use the network configuration to drive the cni network management, e.g: flanneld
			fmt.Sprintf("%s=%s", NetworkConfigurationEnv, getNetworkJSON(c.Network)),
		)
	}

	Binds := []string{}
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		Binds = []string{
			// put the execution binaries and the cni binaries to the host
			fmt.Sprintf("%s:c:/host/opt", path.Join(prefixPath, "/opt")),
			// put the cni configuration to the host
			fmt.Sprintf("%s:c:/host/etc/cni/net.d", path.Join(prefixPath, "/etc/cni/net.d")),
			// put the cni network component configuration to the host
			fmt.Sprintf("%s:c:/host/etc/kube-flannel", path.Join(prefixPath, "/etc/kube-flannel")),
			// exchange resources with other components
			fmt.Sprintf("%s:c:/host/run", path.Join(prefixPath, "/run")),
		}
	}

	RestartPolicy := ""
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		RestartPolicy = "always"
	}

	NetworkMode := "none"
	if host.DockerInfo.OSType == "windows" { // compatible with Windows
		NetworkMode = ""
	}

	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.SystemImages.KubernetesServicesSidecar, c.PrivateRegistriesMap)
	return v3.Process{
		Name:                    services.SidekickContainerName,
		NetworkMode:             NetworkMode,
		RestartPolicy:           RestartPolicy,
		Binds:                   getUniqStringList(Binds),
		Env:                     getUniqStringList(Env),
		Image:                   c.SystemImages.KubernetesServicesSidecar,
		HealthCheck:             v3.HealthCheck{},
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.SidekickContainerName,
		},
		Command: Command,
	}
}

func (c *Cluster) BuildEtcdProcess(host *hosts.Host, etcdHosts []*hosts.Host, prefixPath string, serviceOptions v3.KubernetesServicesOptions) v3.Process {
	nodeName := pki.GetCrtNameForHost(host, pki.EtcdCertName)
	initCluster := ""
	architecture := "amd64"
	if len(etcdHosts) == 0 {
		initCluster = services.GetEtcdInitialCluster(c.EtcdHosts)
		if len(c.EtcdHosts) > 0 {
			architecture = c.EtcdHosts[0].DockerInfo.Architecture
		}
	} else {
		initCluster = services.GetEtcdInitialCluster(etcdHosts)
		architecture = etcdHosts[0].DockerInfo.Architecture
	}

	clusterState := "new"
	if host.ExistingEtcdCluster {
		clusterState = "existing"
	}
	args := []string{
		"/usr/local/bin/etcd",
	}

	// If InternalAddress is not explicitly set, it's set to the same value as Address. This is all good until we deploy on a host with a DNATed public address like AWS, in that case we can't bind to that address so we fall back to 0.0.0.0
	listenAddress := host.InternalAddress
	if host.Address == host.InternalAddress {
		listenAddress = "0.0.0.0"
	}

	CommandArgs := map[string]string{
		"name":                        "etcd-" + host.HostnameOverride,
		"data-dir":                    services.EtcdDataDir,
		"advertise-client-urls":       "https://" + host.InternalAddress + ":2379,https://" + host.InternalAddress + ":4001",
		"listen-client-urls":          "https://" + listenAddress + ":2379",
		"initial-advertise-peer-urls": "https://" + host.InternalAddress + ":2380",
		"listen-peer-urls":            "https://" + listenAddress + ":2380",
		"initial-cluster-token":       "etcd-cluster-1",
		"initial-cluster":             initCluster,
		"initial-cluster-state":       clusterState,
		"trusted-ca-file":             pki.GetCertPath(pki.CACertName),
		"peer-trusted-ca-file":        pki.GetCertPath(pki.CACertName),
		"cert-file":                   pki.GetCertPath(nodeName),
		"key-file":                    pki.GetKeyPath(nodeName),
		"peer-cert-file":              pki.GetCertPath(nodeName),
		"peer-key-file":               pki.GetKeyPath(nodeName),
	}

	Binds := []string{
		fmt.Sprintf("%s:%s:z", path.Join(prefixPath, "/var/lib/etcd"), services.EtcdDataDir),
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
	}

	if serviceOptions.Etcd != nil {
		for k, v := range serviceOptions.Etcd {
			// if the value is empty, we remove that option
			if len(v) == 0 {
				delete(CommandArgs, k)
				continue
			}
			CommandArgs[k] = v
		}
	}

	for arg, value := range c.Services.Etcd.ExtraArgs {
		if _, ok := c.Services.Etcd.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	// adding the old default value from L922 if not present in metadata options or passed by user
	if _, ok := CommandArgs["client-cert-auth"]; !ok {
		args = append(args, "--client-cert-auth")
	}
	if _, ok := CommandArgs["peer-client-cert-auth"]; !ok {
		args = append(args, "--peer-client-cert-auth")
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		args = append(args, cmd)
	}

	Binds = append(Binds, c.Services.Etcd.ExtraBinds...)
	healthCheck := v3.HealthCheck{
		URL: fmt.Sprintf("https://%s:2379/health", host.InternalAddress),
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.Etcd.Image, c.PrivateRegistriesMap)

	// Determine etcd version for correct etcdctl environment variables
	etcdTag, err := util.GetImageTagFromImage(c.Services.Etcd.Image)
	if err != nil {
		logrus.Warn(err)
	}
	etcdSemVer, err := util.StrToSemVer(etcdTag)
	if err != nil {
		logrus.Warn(err)
	}
	maxEtcdOldEnvSemVer, err := util.StrToSemVer(MaxEtcdOldEnvVersion)
	if err != nil {
		logrus.Warn(err)
	}

	// Configure default etcdctl environment variables
	Env := []string{}
	Env = append(Env, "ETCDCTL_API=3")
	Env = append(Env, fmt.Sprintf("ETCDCTL_CACERT=%s", pki.GetCertPath(pki.CACertName)))
	Env = append(Env, fmt.Sprintf("ETCDCTL_CERT=%s", pki.GetCertPath(nodeName)))
	Env = append(Env, fmt.Sprintf("ETCDCTL_KEY=%s", pki.GetKeyPath(nodeName)))

	// Apply old configuration to avoid replacing etcd container
	if etcdSemVer.LessThan(*maxEtcdOldEnvSemVer) {
		logrus.Debugf("Version [%s] is less than version [%s]", etcdSemVer, maxEtcdOldEnvSemVer)
		Env = append(Env, fmt.Sprintf("ETCDCTL_ENDPOINT=https://%s:2379", listenAddress))
	} else {
		logrus.Debugf("Version [%s] is equal or higher than version [%s]", etcdSemVer, maxEtcdOldEnvSemVer)
		// Point etcdctl to localhost in case we have listen all (0.0.0.0) configured
		if listenAddress == "0.0.0.0" {
			Env = append(Env, "ETCDCTL_ENDPOINTS=https://127.0.0.1:2379")
			// If internal address is configured, set endpoint to that address as well
		} else {
			Env = append(Env, fmt.Sprintf("ETCDCTL_ENDPOINTS=https://%s:2379", listenAddress))
		}
	}

	if architecture == "aarch64" {
		architecture = "arm64"
	}
	Env = append(Env, fmt.Sprintf("ETCD_UNSUPPORTED_ARCH=%s", architecture))

	Env = append(Env, c.Services.Etcd.ExtraEnv...)
	var user string
	if c.Services.Etcd.UID != 0 && c.Services.Etcd.GID != 0 {
		user = fmt.Sprintf("%d:%d", c.Services.Etcd.UID, c.Services.Etcd.UID)
	}
	return v3.Process{
		Name:                    services.EtcdContainerName,
		Args:                    args,
		Binds:                   getUniqStringList(Binds),
		Env:                     Env,
		User:                    user,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Etcd.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			services.ContainerNameLabel: services.EtcdContainerName,
		},
	}
}

func BuildPortChecksFromPortList(host *hosts.Host, portList []string, proto string) []v3.PortCheck {
	portChecks := []v3.PortCheck{}
	for _, port := range portList {
		intPort, _ := strconv.Atoi(port)
		portChecks = append(portChecks, v3.PortCheck{
			Address:  host.Address,
			Port:     intPort,
			Protocol: proto,
		})
	}
	return portChecks
}

func (c *Cluster) GetKubernetesServicesOptions(osType string, data map[string]*v3.KubernetesServicesOptions) (v3.KubernetesServicesOptions, error) {
	if osType == "windows" {
		if svcOption, ok := data["k8s-windows-service-options"]; ok {
			return *svcOption, nil
		}
	} else {
		if svcOption, ok := data["k8s-service-options"]; ok {
			return *svcOption, nil
		}
	}
	return c.getDefaultKubernetesServicesOptions(osType)
}

func (c *Cluster) getDefaultKubernetesServicesOptions(osType string) (v3.KubernetesServicesOptions, error) {
	var serviceOptionsTemplate map[string]v3.KubernetesServicesOptions
	switch osType {
	case "windows":
		serviceOptionsTemplate = metadata.K8sVersionToWindowsServiceOptions
	default:
		serviceOptionsTemplate = metadata.K8sVersionToServiceOptions
	}

	// read service options from most specific cluster version first
	// Example c.Version: v1.16.3-rancher1-1
	logrus.Debugf("getDefaultKubernetesServicesOptions: getting serviceOptions for cluster version [%s]", c.Version)
	if serviceOptions, ok := serviceOptionsTemplate[c.Version]; ok {
		logrus.Debugf("getDefaultKubernetesServicesOptions: serviceOptions found for cluster version [%s]", c.Version)
		logrus.Tracef("getDefaultKubernetesServicesOptions: [%s] serviceOptions [%v]", c.Version, serviceOptions)
		return serviceOptions, nil
	}

	// Get vX.X from cluster version
	// Example clusterMajorVersion: v1.16
	clusterMajorVersion := util.GetTagMajorVersion(c.Version)
	// Retrieve image tag from Kubernetes image
	// Example k8sImageTag: v1.16.3-rancher1
	k8sImageTag, err := util.GetImageTagFromImage(c.SystemImages.Kubernetes)
	if err != nil {
		logrus.Warn(err)
	}

	// Example k8sImageMajorVersion: v1.16
	k8sImageMajorVersion := util.GetTagMajorVersion(k8sImageTag)

	// Image tag version from Kubernetes image takes precedence over cluster version
	if clusterMajorVersion != k8sImageMajorVersion && k8sImageMajorVersion != "" {
		logrus.Debugf("getDefaultKubernetesServicesOptions: cluster major version: [%s] is not equal to kubernetes image major version: [%s], setting cluster major version to [%s]", clusterMajorVersion, k8sImageMajorVersion, k8sImageMajorVersion)
		clusterMajorVersion = k8sImageMajorVersion
	}

	if serviceOptions, ok := serviceOptionsTemplate[clusterMajorVersion]; ok {
		logrus.Debugf("getDefaultKubernetesServicesOptions: serviceOptions found for cluster major version [%s]", clusterMajorVersion)
		logrus.Tracef("getDefaultKubernetesServicesOptions: [%s] serviceOptions [%v]", clusterMajorVersion, serviceOptions)
		return serviceOptions, nil
	}

	return v3.KubernetesServicesOptions{}, fmt.Errorf("getDefaultKubernetesServicesOptions: No serviceOptions found for cluster version [%s] or cluster major version [%s]", c.Version, clusterMajorVersion)
}

func getCloudConfigChecksum(config string) string {
	configByteSum := md5.Sum([]byte(config))
	return fmt.Sprintf("%x", configByteSum)
}

func getUniqStringList(l []string) []string {
	m := map[string]bool{}
	ul := []string{}
	for _, k := range l {
		if _, ok := m[k]; !ok {
			m[k] = true
			ul = append(ul, k)
		}
	}
	return ul
}

func (c *Cluster) getRKEToolsEntryPoint() string {
	v := strings.Split(c.SystemImages.KubernetesServicesSidecar, ":")
	last := v[len(v)-1]

	sv, err := util.StrToSemVer(last)
	if err != nil {
		return DefaultToolsEntrypoint
	}
	svdefault, err := util.StrToSemVer(DefaultToolsEntrypointVersion)
	if err != nil {
		return DefaultToolsEntrypoint
	}

	if sv.LessThan(*svdefault) {
		return LegacyToolsEntrypoint
	}
	return DefaultToolsEntrypoint
}

func getNetworkJSON(netconfig v3.NetworkConfig) string {
	ret, err := json.Marshal(netconfig)
	if err != nil {
		return "{}"
	}
	return string(ret)
}
