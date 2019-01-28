package cluster

import (
	"context"
	"crypto/md5"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"

	b64 "encoding/base64"

	ref "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/cloudprovider/aws"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	EtcdPathPrefix     = "/registry"
	ContainerNameLabel = "io.rancher.rke.container.name"
	CloudConfigSumEnv  = "RKE_CLOUD_CONFIG_CHECKSUM"

	DefaultToolsEntrypoint        = "/opt/rke-tools/entrypoint.sh"
	DefaultToolsEntrypointVersion = "0.1.13"
	LegacyToolsEntrypoint         = "/opt/rke/entrypoint.sh"

	KubeletDockerConfigEnv     = "RKE_KUBELET_DOCKER_CONFIG"
	KubeletDockerConfigFileEnv = "RKE_KUBELET_DOCKER_FILE"
	KubeletDockerConfigPath    = "/var/lib/kubelet/config.json"
)

var admissionControlOptionNames = []string{"enable-admission-plugins", "admission-control"}

func GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, hostsInfoMap map[string]types.Info) (v3.RKEPlan, error) {
	clusterPlan := v3.RKEPlan{}
	myCluster, err := InitClusterObject(ctx, rkeConfig, ExternalFlags{})
	if err != nil {
		return clusterPlan, err
	}
	// rkeConfig.Nodes are already unique. But they don't have role flags. So I will use the parsed cluster.Hosts to make use of the role flags.
	uniqHosts := hosts.GetUniqueHostList(myCluster.EtcdHosts, myCluster.ControlPlaneHosts, myCluster.WorkerHosts)
	for _, host := range uniqHosts {
		host.DockerInfo = hostsInfoMap[host.Address]
		clusterPlan.Nodes = append(clusterPlan.Nodes, BuildRKEConfigNodePlan(ctx, myCluster, host, hostsInfoMap[host.Address]))
	}
	return clusterPlan, nil
}

func BuildRKEConfigNodePlan(ctx context.Context, myCluster *Cluster, host *hosts.Host, hostDockerInfo types.Info) v3.RKEConfigNodePlan {
	prefixPath := hosts.GetPrefixPath(hostDockerInfo.OperatingSystem, myCluster.PrefixPath)
	processes := map[string]v3.Process{}
	portChecks := []v3.PortCheck{}
	// Everybody gets a sidecar and a kubelet..
	processes[services.SidekickContainerName] = myCluster.BuildSidecarProcess()
	processes[services.KubeletContainerName] = myCluster.BuildKubeletProcess(host, prefixPath)
	processes[services.KubeproxyContainerName] = myCluster.BuildKubeProxyProcess(host, prefixPath)

	portChecks = append(portChecks, BuildPortChecksFromPortList(host, WorkerPortList, ProtocolTCP)...)
	// Do we need an nginxProxy for this one ?
	if !host.IsControl {
		processes[services.NginxProxyContainerName] = myCluster.BuildProxyProcess()
	}
	if host.IsControl {
		processes[services.KubeAPIContainerName] = myCluster.BuildKubeAPIProcess(host, prefixPath)
		processes[services.KubeControllerContainerName] = myCluster.BuildKubeControllerProcess(prefixPath)
		processes[services.SchedulerContainerName] = myCluster.BuildSchedulerProcess(prefixPath)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, ControlPlanePortList, ProtocolTCP)...)
	}
	if host.IsEtcd {
		processes[services.EtcdContainerName] = myCluster.BuildEtcdProcess(host, myCluster.EtcdReadyHosts, prefixPath)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, EtcdPortList, ProtocolTCP)...)
	}
	cloudConfig := v3.File{
		Name:     cloudConfigFileName,
		Contents: b64.StdEncoding.EncodeToString([]byte(myCluster.CloudConfigFile)),
	}
	return v3.RKEConfigNodePlan{
		Address:    host.Address,
		Processes:  processes,
		PortChecks: portChecks,
		Files:      []v3.File{cloudConfig},
		Annotations: map[string]string{
			k8s.ExternalAddressAnnotation: host.Address,
			k8s.InternalAddressAnnotation: host.InternalAddress,
		},
		Labels: host.ToAddLabels,
	}
}

func (c *Cluster) BuildKubeAPIProcess(host *hosts.Host, prefixPath string) v3.Process {
	// check if external etcd is used
	etcdConnectionString := services.GetEtcdConnString(c.EtcdHosts)
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
	baseEnabledAdmissionPlugins := []string{
		"DefaultStorageClass",
		"DefaultTolerationSeconds",
		"LimitRanger",
		"NamespaceLifecycle",
		"NodeRestriction",
		"PersistentVolumeLabel",
		"ResourceQuota",
		"ServiceAccount",
	}
	CommandArgs := map[string]string{
		"allow-privileged":                   "true",
		"anonymous-auth":                     "false",
		"bind-address":                       "0.0.0.0",
		"client-ca-file":                     pki.GetCertPath(pki.CACertName),
		"cloud-provider":                     c.CloudProvider.Name,
		"etcd-cafile":                        etcdCAClientCert,
		"etcd-certfile":                      etcdClientCert,
		"etcd-keyfile":                       etcdClientKey,
		"etcd-prefix":                        etcdPathPrefix,
		"etcd-servers":                       etcdConnectionString,
		"insecure-port":                      "0",
		"kubelet-client-certificate":         pki.GetCertPath(pki.KubeAPICertName),
		"kubelet-client-key":                 pki.GetKeyPath(pki.KubeAPICertName),
		"kubelet-preferred-address-types":    "InternalIP,ExternalIP,Hostname",
		"profiling":                          "false",
		"proxy-client-cert-file":             pki.GetCertPath(pki.APIProxyClientCertName),
		"proxy-client-key-file":              pki.GetKeyPath(pki.APIProxyClientCertName),
		"requestheader-allowed-names":        pki.APIProxyClientCertName,
		"requestheader-client-ca-file":       pki.GetCertPath(pki.RequestHeaderCACertName),
		"requestheader-extra-headers-prefix": "X-Remote-Extra-",
		"requestheader-group-headers":        "X-Remote-Group",
		"requestheader-username-headers":     "X-Remote-User",
		"repair-malformed-updates":           "false",
		"secure-port":                        "6443",
		"service-account-key-file":           pki.GetKeyPath(pki.ServiceAccountTokenKeyName),
		"service-account-lookup":             "true",
		"service-cluster-ip-range":           c.Services.KubeAPI.ServiceClusterIPRange,
		"service-node-port-range":            c.Services.KubeAPI.ServiceNodePortRange,
		"storage-backend":                    "etcd3",
		"tls-cert-file":                      pki.GetCertPath(pki.KubeAPICertName),
		"tls-private-key-file":               pki.GetKeyPath(pki.KubeAPICertName),
	}
	if len(c.CloudProvider.Name) > 0 && c.CloudProvider.Name != aws.AWSCloudProviderName {
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
	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
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
	if getTagMajorVersion(c.Version) == "v1.8" {
		CommandArgs["apiserver-count"] = strconv.Itoa(len(c.ControlPlaneHosts))
	}

	if c.Authorization.Mode == services.RBACAuthorizationMode {
		CommandArgs["authorization-mode"] = "Node,RBAC"
	}

	if len(host.InternalAddress) > 0 && net.ParseIP(host.InternalAddress) != nil {
		CommandArgs["advertise-address"] = host.InternalAddress
	}

	// PodSecurityPolicy
	if c.Services.KubeAPI.PodSecurityPolicy {
		CommandArgs["runtime-config"] = "extensions/v1beta1/podsecuritypolicy=true"
		baseEnabledAdmissionPlugins = append(baseEnabledAdmissionPlugins, "PodSecurityPolicy")
	}

	// AlwaysPullImages
	if c.Services.KubeAPI.AlwaysPullImages {
		baseEnabledAdmissionPlugins = append(baseEnabledAdmissionPlugins, "AlwaysPullImages")
	}

	// Admission control plugins
	// Resolution order:
	//   k8s_defaults.go K8sVersionServiceOptions
	//   enabledAdmissionPlugins
	//   cluster.yml extra_args overwrites it all
	for _, optionName := range admissionControlOptionNames {
		if _, ok := CommandArgs[optionName]; ok {
			enabledAdmissionPlugins := strings.Split(CommandArgs[optionName], ",")
			enabledAdmissionPlugins = append(enabledAdmissionPlugins, baseEnabledAdmissionPlugins...)

			// Join unique slice as arg
			CommandArgs[optionName] = strings.Join(util.UniqueStringSlice(enabledAdmissionPlugins), ",")
			break
		}
	}
	if c.Services.KubeAPI.PodSecurityPolicy {
		CommandArgs["runtime-config"] = "extensions/v1beta1/podsecuritypolicy=true"
		for _, optionName := range admissionControlOptionNames {
			if _, ok := CommandArgs[optionName]; ok {
				CommandArgs[optionName] = CommandArgs[optionName] + ",PodSecurityPolicy"
				break
			}
		}
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
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
			ContainerNameLabel: services.KubeAPIContainerName,
		},
	}
}

func (c *Cluster) BuildKubeControllerProcess(prefixPath string) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-controller-manager",
	}

	CommandArgs := map[string]string{
		"address":                          "127.0.0.1",
		"allow-untagged-cloud":             "true",
		"allocate-node-cidrs":              "true",
		"cloud-provider":                   c.CloudProvider.Name,
		"cluster-cidr":                     c.ClusterCIDR,
		"configure-cloud-routes":           "false",
		"enable-hostpath-provisioner":      "false",
		"kubeconfig":                       pki.GetConfigPath(pki.KubeControllerCertName),
		"leader-elect":                     "true",
		"node-monitor-grace-period":        "40s",
		"pod-eviction-timeout":             "5m0s",
		"profiling":                        "false",
		"root-ca-file":                     pki.GetCertPath(pki.CACertName),
		"service-account-private-key-file": pki.GetKeyPath(pki.ServiceAccountTokenKeyName),
		"service-cluster-ip-range":         c.Services.KubeController.ServiceClusterIPRange,
		"terminated-pod-gc-threshold":      "1000",
		"v":                                "2",
	}
	// Best security practice is to listen on localhost, but DinD uses private container network instead of Host.
	if c.DinD {
		CommandArgs["address"] = "0.0.0.0"
	}
	if len(c.CloudProvider.Name) > 0 && c.CloudProvider.Name != aws.AWSCloudProviderName {
		CommandArgs["cloud-config"] = cloudConfigFileName
	}
	if len(c.CloudProvider.Name) > 0 {
		c.Services.KubeController.ExtraEnv = append(
			c.Services.KubeController.ExtraEnv,
			fmt.Sprintf("%s=%s", CloudConfigSumEnv, getCloudConfigChecksum(c.CloudConfigFile)))
	}
	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
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
			ContainerNameLabel: services.KubeControllerContainerName,
		},
	}
}

func (c *Cluster) BuildKubeletProcess(host *hosts.Host, prefixPath string) v3.Process {

	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kubelet",
	}

	CommandArgs := map[string]string{
		"address":                           "0.0.0.0",
		"allow-privileged":                  "true",
		"anonymous-auth":                    "false",
		"authentication-token-webhook":      "true",
		"cgroups-per-qos":                   "True",
		"client-ca-file":                    pki.GetCertPath(pki.CACertName),
		"cloud-provider":                    c.CloudProvider.Name,
		"cluster-dns":                       c.ClusterDNSServer,
		"cluster-domain":                    c.ClusterDomain,
		"cni-bin-dir":                       "/opt/cni/bin",
		"cni-conf-dir":                      "/etc/cni/net.d",
		"enforce-node-allocatable":          "",
		"event-qps":                         "0",
		"fail-swap-on":                      strconv.FormatBool(c.Services.Kubelet.FailSwapOn),
		"hostname-override":                 host.HostnameOverride,
		"kubeconfig":                        pki.GetConfigPath(pki.KubeNodeCertName),
		"make-iptables-util-chains":         "true",
		"network-plugin":                    "cni",
		"pod-infra-container-image":         c.Services.Kubelet.InfraContainerImage,
		"read-only-port":                    "0",
		"resolv-conf":                       "/etc/resolv.conf",
		"root-dir":                          path.Join(prefixPath, "/var/lib/kubelet"),
		"streaming-connection-idle-timeout": "30m",
		"volume-plugin-dir":                 "/var/lib/kubelet/volumeplugins",
		"v":                                 "2",
	}
	if host.IsControl && !host.IsWorker {
		CommandArgs["register-with-taints"] = unschedulableControlTaint
	}
	if host.Address != host.InternalAddress {
		CommandArgs["node-ip"] = host.InternalAddress
	}
	if len(c.CloudProvider.Name) > 0 && c.CloudProvider.Name != aws.AWSCloudProviderName {
		CommandArgs["cloud-config"] = cloudConfigFileName
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
	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
	if serviceOptions.Kubelet != nil {
		for k, v := range serviceOptions.Kubelet {
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

	for arg, value := range c.Services.Kubelet.ExtraArgs {
		if _, ok := c.Services.Kubelet.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
	}

	for arg, value := range CommandArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}

	Binds = append(Binds, c.Services.Kubelet.ExtraBinds...)

	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(true, services.KubeletPort),
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.Kubelet.Image, c.PrivateRegistriesMap)

	return v3.Process{
		Name:                    services.KubeletContainerName,
		Command:                 Command,
		VolumesFrom:             VolumesFrom,
		Binds:                   getUniqStringList(Binds),
		Env:                     getUniqStringList(c.Services.Kubelet.ExtraEnv),
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Kubelet.Image,
		PidMode:                 "host",
		Privileged:              true,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			ContainerNameLabel: services.KubeletContainerName,
		},
	}
}

func (c *Cluster) BuildKubeProxyProcess(host *hosts.Host, prefixPath string) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-proxy",
	}

	CommandArgs := map[string]string{
		"cluster-cidr":         c.ClusterCIDR,
		"v":                    "2",
		"healthz-bind-address": "127.0.0.1",
		"hostname-override":    host.HostnameOverride,
		"kubeconfig":           pki.GetConfigPath(pki.KubeProxyCertName),
	}
	// Best security practice is to listen on localhost, but DinD uses private container network instead of Host.
	if c.DinD {
		CommandArgs["healthz-bind-address"] = "0.0.0.0"
	}
	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
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

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
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
		Env:                     c.Services.Kubeproxy.ExtraEnv,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		PidMode:                 "host",
		Privileged:              true,
		HealthCheck:             healthCheck,
		Image:                   c.Services.Kubeproxy.Image,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			ContainerNameLabel: services.KubeproxyContainerName,
		},
	}
}

func (c *Cluster) BuildProxyProcess() v3.Process {
	nginxProxyEnv := ""
	for i, host := range c.ControlPlaneHosts {
		nginxProxyEnv += fmt.Sprintf("%s", host.InternalAddress)
		if i < (len(c.ControlPlaneHosts) - 1) {
			nginxProxyEnv += ","
		}
	}
	Env := []string{fmt.Sprintf("%s=%s", services.NginxProxyEnvName, nginxProxyEnv)}

	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.SystemImages.NginxProxy, c.PrivateRegistriesMap)
	return v3.Process{
		Name: services.NginxProxyContainerName,
		Env:  Env,
		// we do this to force container update when CP hosts change.
		Args:                    Env,
		Command:                 []string{"nginx-proxy"},
		NetworkMode:             "host",
		RestartPolicy:           "always",
		HealthCheck:             v3.HealthCheck{},
		Image:                   c.SystemImages.NginxProxy,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			ContainerNameLabel: services.NginxProxyContainerName,
		},
	}
}

func (c *Cluster) BuildSchedulerProcess(prefixPath string) v3.Process {
	Command := []string{
		c.getRKEToolsEntryPoint(),
		"kube-scheduler",
	}

	CommandArgs := map[string]string{
		"leader-elect": "true",
		"v":            "2",
		"address":      "127.0.0.1",
		"profiling":    "false",
		"kubeconfig":   pki.GetConfigPath(pki.KubeSchedulerCertName),
	}

	// Best security practice is to listen on localhost, but DinD uses private container network instead of Host.
	if c.DinD {
		CommandArgs["address"] = "0.0.0.0"
	}

	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
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
			ContainerNameLabel: services.SchedulerContainerName,
		},
	}
}

func (c *Cluster) BuildSidecarProcess() v3.Process {
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.SystemImages.KubernetesServicesSidecar, c.PrivateRegistriesMap)
	return v3.Process{
		Name:                    services.SidekickContainerName,
		NetworkMode:             "none",
		Image:                   c.SystemImages.KubernetesServicesSidecar,
		HealthCheck:             v3.HealthCheck{},
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			ContainerNameLabel: services.SidekickContainerName,
		},
		Command: []string{"/bin/bash"},
	}
}

func (c *Cluster) BuildEtcdProcess(host *hosts.Host, etcdHosts []*hosts.Host, prefixPath string) v3.Process {
	nodeName := pki.GetEtcdCrtName(host.InternalAddress)
	initCluster := ""
	if len(etcdHosts) == 0 {
		initCluster = services.GetEtcdInitialCluster(c.EtcdHosts)
	} else {
		initCluster = services.GetEtcdInitialCluster(etcdHosts)
	}

	clusterState := "new"
	if host.ExistingEtcdCluster {
		clusterState = "existing"
	}
	args := []string{
		"/usr/local/bin/etcd",
		"--peer-client-cert-auth",
		"--client-cert-auth",
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

	for arg, value := range c.Services.Etcd.ExtraArgs {
		if _, ok := c.Services.Etcd.ExtraArgs[arg]; ok {
			CommandArgs[arg] = value
		}
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

	Env := []string{}
	Env = append(Env, "ETCDCTL_API=3")
	Env = append(Env, fmt.Sprintf("ETCDCTL_ENDPOINT=https://%s:2379", listenAddress))
	Env = append(Env, fmt.Sprintf("ETCDCTL_CACERT=%s", pki.GetCertPath(pki.CACertName)))
	Env = append(Env, fmt.Sprintf("ETCDCTL_CERT=%s", pki.GetCertPath(nodeName)))
	Env = append(Env, fmt.Sprintf("ETCDCTL_KEY=%s", pki.GetKeyPath(nodeName)))

	Env = append(Env, c.Services.Etcd.ExtraEnv...)

	return v3.Process{
		Name:                    services.EtcdContainerName,
		Args:                    args,
		Binds:                   getUniqStringList(Binds),
		Env:                     Env,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Etcd.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
		Labels: map[string]string{
			ContainerNameLabel: services.EtcdContainerName,
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

func (c *Cluster) GetKubernetesServicesOptions() v3.KubernetesServicesOptions {
	clusterMajorVersion := getTagMajorVersion(c.Version)
	NamedkK8sImage, _ := ref.ParseNormalizedNamed(c.SystemImages.Kubernetes)

	k8sImageTag := NamedkK8sImage.(ref.Tagged).Tag()
	k8sImageMajorVersion := getTagMajorVersion(k8sImageTag)

	if clusterMajorVersion != k8sImageMajorVersion && k8sImageMajorVersion != "" {
		clusterMajorVersion = k8sImageMajorVersion
	}

	serviceOptions, ok := v3.K8sVersionServiceOptions[clusterMajorVersion]
	if ok {
		return serviceOptions
	}
	return v3.KubernetesServicesOptions{}
}

func getTagMajorVersion(tag string) string {
	splitTag := strings.Split(tag, ".")
	if len(splitTag) < 2 {
		return ""
	}
	return strings.Join(splitTag[:2], ".")
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

	logrus.Debugf("Extracted version [%s] from image [%s]", last, c.SystemImages.KubernetesServicesSidecar)

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
