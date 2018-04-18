package cluster

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	b64 "encoding/base64"

	ref "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	EtcdPathPrefix = "/registry"
	B2DOS          = "Boot2Docker"
	B2DPrefixPath  = "/mnt/sda1/rke"
	ROS            = "RancherOS"
	ROSPrefixPath  = "/opt/rke"
)

func GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, hostsInfoMap map[string]types.Info) (v3.RKEPlan, error) {
	clusterPlan := v3.RKEPlan{}
	myCluster, _ := ParseCluster(ctx, rkeConfig, "", "", nil, nil, nil)
	// rkeConfig.Nodes are already unique. But they don't have role flags. So I will use the parsed cluster.Hosts to make use of the role flags.
	uniqHosts := hosts.GetUniqueHostList(myCluster.EtcdHosts, myCluster.ControlPlaneHosts, myCluster.WorkerHosts)
	for _, host := range uniqHosts {
		clusterPlan.Nodes = append(clusterPlan.Nodes, BuildRKEConfigNodePlan(ctx, myCluster, host, hostsInfoMap[host.Address]))
	}
	return clusterPlan, nil
}

func BuildRKEConfigNodePlan(ctx context.Context, myCluster *Cluster, host *hosts.Host, hostDockerInfo types.Info) v3.RKEConfigNodePlan {
	prefixPath := myCluster.getPrefixPath(hostDockerInfo.OperatingSystem)
	processes := map[string]v3.Process{}
	portChecks := []v3.PortCheck{}
	// Everybody gets a sidecar and a kubelet..
	processes[services.SidekickContainerName] = myCluster.BuildSidecarProcess()
	processes[services.KubeletContainerName] = myCluster.BuildKubeletProcess(host, prefixPath)
	processes[services.KubeproxyContainerName] = myCluster.BuildKubeProxyProcess(prefixPath)

	portChecks = append(portChecks, BuildPortChecksFromPortList(host, WorkerPortList, ProtocolTCP)...)
	// Do we need an nginxProxy for this one ?
	if !host.IsControl {
		processes[services.NginxProxyContainerName] = myCluster.BuildProxyProcess()
	}
	if host.IsControl {
		processes[services.KubeAPIContainerName] = myCluster.BuildKubeAPIProcess(prefixPath)
		processes[services.KubeControllerContainerName] = myCluster.BuildKubeControllerProcess(prefixPath)
		processes[services.SchedulerContainerName] = myCluster.BuildSchedulerProcess(prefixPath)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, ControlPlanePortList, ProtocolTCP)...)
	}
	if host.IsEtcd {
		processes[services.EtcdContainerName] = myCluster.BuildEtcdProcess(host, myCluster.EtcdReadyHosts, prefixPath)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, EtcdPortList, ProtocolTCP)...)
	}
	cloudConfig := v3.File{
		Name:     CloudConfigPath,
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

func (c *Cluster) BuildKubeAPIProcess(prefixPath string) v3.Process {
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
		"/opt/rke/entrypoint.sh",
		"kube-apiserver",
	}

	CommandArgs := map[string]string{
		"insecure-bind-address":           "127.0.0.1",
		"bind-address":                    "0.0.0.0",
		"insecure-port":                   "0",
		"secure-port":                     "6443",
		"cloud-provider":                  c.CloudProvider.Name,
		"allow-privileged":                "true",
		"kubelet-preferred-address-types": "InternalIP,ExternalIP,Hostname",
		"service-cluster-ip-range":        c.Services.KubeAPI.ServiceClusterIPRange,
		"admission-control":               "ServiceAccount,NamespaceLifecycle,LimitRanger,PersistentVolumeLabel,DefaultStorageClass,ResourceQuota,DefaultTolerationSeconds",
		"storage-backend":                 "etcd3",
		"client-ca-file":                  pki.GetCertPath(pki.CACertName),
		"tls-cert-file":                   pki.GetCertPath(pki.KubeAPICertName),
		"tls-private-key-file":            pki.GetKeyPath(pki.KubeAPICertName),
		"kubelet-client-certificate":      pki.GetCertPath(pki.KubeAPICertName),
		"kubelet-client-key":              pki.GetKeyPath(pki.KubeAPICertName),
		"service-account-key-file":        pki.GetKeyPath(pki.KubeAPICertName),
	}
	if len(c.CloudProvider.Name) > 0 {
		CommandArgs["cloud-config"] = CloudConfigPath
	}
	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
	if serviceOptions.KubeAPI != nil {
		for k, v := range serviceOptions.KubeAPI {
			CommandArgs[k] = v
		}
	}

	args := []string{
		"--etcd-cafile=" + etcdCAClientCert,
		"--etcd-certfile=" + etcdClientCert,
		"--etcd-keyfile=" + etcdClientKey,
		"--etcd-servers=" + etcdConnectionString,
		"--etcd-prefix=" + etcdPathPrefix,
	}

	if c.Authorization.Mode == services.RBACAuthorizationMode {
		CommandArgs["authorization-mode"] = "Node,RBAC"
	}
	if c.Services.KubeAPI.PodSecurityPolicy {
		CommandArgs["runtime-config"] = "extensions/v1beta1/podsecuritypolicy=true"
		CommandArgs["admission-control"] = CommandArgs["admission-control"] + ",PodSecurityPolicy"
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
		Args:                    args,
		VolumesFrom:             VolumesFrom,
		Binds:                   Binds,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.KubeAPI.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
	}
}

func (c *Cluster) BuildKubeControllerProcess(prefixPath string) v3.Process {
	Command := []string{
		"/opt/rke/entrypoint.sh",
		"kube-controller-manager",
	}

	CommandArgs := map[string]string{
		"address":                     "0.0.0.0",
		"cloud-provider":              c.CloudProvider.Name,
		"allow-untagged-cloud":        "true",
		"configure-cloud-routes":      "false",
		"leader-elect":                "true",
		"kubeconfig":                  pki.GetConfigPath(pki.KubeControllerCertName),
		"enable-hostpath-provisioner": "false",
		"node-monitor-grace-period":   "40s",
		"pod-eviction-timeout":        "5m0s",
		"v": "2",
		"allocate-node-cidrs":              "true",
		"cluster-cidr":                     c.ClusterCIDR,
		"service-cluster-ip-range":         c.Services.KubeController.ServiceClusterIPRange,
		"service-account-private-key-file": pki.GetKeyPath(pki.KubeAPICertName),
		"root-ca-file":                     pki.GetCertPath(pki.CACertName),
	}
	if len(c.CloudProvider.Name) > 0 {
		CommandArgs["cloud-config"] = CloudConfigPath
	}

	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
	if serviceOptions.KubeController != nil {
		for k, v := range serviceOptions.KubeController {
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
		Binds:                   Binds,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.KubeController.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
	}
}

func (c *Cluster) BuildKubeletProcess(host *hosts.Host, prefixPath string) v3.Process {

	Command := []string{
		"/opt/rke/entrypoint.sh",
		"kubelet",
	}

	CommandArgs := map[string]string{
		"v":                         "2",
		"address":                   "0.0.0.0",
		"cadvisor-port":             "0",
		"read-only-port":            "0",
		"cluster-domain":            c.ClusterDomain,
		"pod-infra-container-image": c.Services.Kubelet.InfraContainerImage,
		"cgroups-per-qos":           "True",
		"enforce-node-allocatable":  "",
		"hostname-override":         host.HostnameOverride,
		"cluster-dns":               c.ClusterDNSServer,
		"network-plugin":            "cni",
		"cni-conf-dir":              "/etc/cni/net.d",
		"cni-bin-dir":               "/opt/cni/bin",
		"resolv-conf":               "/etc/resolv.conf",
		"allow-privileged":          "true",
		"cloud-provider":            c.CloudProvider.Name,
		"kubeconfig":                pki.GetConfigPath(pki.KubeNodeCertName),
		"client-ca-file":            pki.GetCertPath(pki.CACertName),
		"anonymous-auth":            "false",
		"volume-plugin-dir":         "/var/lib/kubelet/volumeplugins",
		"fail-swap-on":              strconv.FormatBool(c.Services.Kubelet.FailSwapOn),
		"root-dir":                  path.Join(prefixPath, "/var/lib/kubelet"),
	}
	if host.Address != host.InternalAddress {
		CommandArgs["node-ip"] = host.InternalAddress
	}
	if len(c.CloudProvider.Name) > 0 {
		CommandArgs["cloud-config"] = CloudConfigPath
	}

	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
	if serviceOptions.Kubelet != nil {
		for k, v := range serviceOptions.Kubelet {
			CommandArgs[k] = v
		}
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(prefixPath, "/etc/kubernetes")),
		"/etc/cni:/etc/cni:ro,z",
		"/opt/cni:/opt/cni:ro,z",
		fmt.Sprintf("%s:/var/lib/cni:z", path.Join(prefixPath, "/var/lib/cni")),
		"/etc/resolv.conf:/etc/resolv.conf",
		"/sys:/sys:rprivate",
		host.DockerInfo.DockerRootDir + ":" + host.DockerInfo.DockerRootDir + ":rw,rslave,z",
		fmt.Sprintf("%s:%s:shared,z", path.Join(prefixPath, "/var/lib/kubelet"), path.Join(prefixPath, "/var/lib/kubelet")),
		fmt.Sprintf("%s:%s:shared,z", path.Join(prefixPath, "/var/lib/rancher"), path.Join(prefixPath, "/var/lib/rancher")),
		"/var/run:/var/run:rw,rprivate",
		"/run:/run:rprivate",
		fmt.Sprintf("%s:/etc/ceph", path.Join(prefixPath, "/etc/ceph")),
		"/dev:/host/dev:rprivate",
		fmt.Sprintf("%s:/var/log/containers:z", path.Join(prefixPath, "/var/log/containers")),
		fmt.Sprintf("%s:/var/log/pods:z", path.Join(prefixPath, "/var/log/pods")),
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
		Binds:                   Binds,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Kubelet.Image,
		PidMode:                 "host",
		Privileged:              true,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
	}
}

func (c *Cluster) BuildKubeProxyProcess(prefixPath string) v3.Process {
	Command := []string{
		"/opt/rke/entrypoint.sh",
		"kube-proxy",
	}

	CommandArgs := map[string]string{
		"v": "2",
		"healthz-bind-address": "0.0.0.0",
		"kubeconfig":           pki.GetConfigPath(pki.KubeProxyCertName),
	}

	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
	if serviceOptions.Kubeproxy != nil {
		for k, v := range serviceOptions.Kubeproxy {
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
		Name:          services.KubeproxyContainerName,
		Command:       Command,
		VolumesFrom:   VolumesFrom,
		Binds:         Binds,
		NetworkMode:   "host",
		RestartPolicy: "always",
		PidMode:       "host",
		Privileged:    true,
		HealthCheck:   healthCheck,
		Image:         c.Services.Kubeproxy.Image,
		ImageRegistryAuthConfig: registryAuthConfig,
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
		Name:          services.NginxProxyContainerName,
		Env:           Env,
		Args:          Env,
		NetworkMode:   "host",
		RestartPolicy: "always",
		HealthCheck:   v3.HealthCheck{},
		Image:         c.SystemImages.NginxProxy,
		ImageRegistryAuthConfig: registryAuthConfig,
	}
}

func (c *Cluster) BuildSchedulerProcess(prefixPath string) v3.Process {
	Command := []string{
		"/opt/rke/entrypoint.sh",
		"kube-scheduler",
	}

	CommandArgs := map[string]string{
		"leader-elect": "true",
		"v":            "2",
		"address":      "0.0.0.0",
		"kubeconfig":   pki.GetConfigPath(pki.KubeSchedulerCertName),
	}

	// check if our version has specific options for this component
	serviceOptions := c.GetKubernetesServicesOptions()
	if serviceOptions.Scheduler != nil {
		for k, v := range serviceOptions.Scheduler {
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
		Binds:                   Binds,
		VolumesFrom:             VolumesFrom,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Scheduler.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
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

	CommandArgs := map[string]string{
		"name":                        "etcd-" + host.HostnameOverride,
		"data-dir":                    "/var/lib/rancher/etcd",
		"advertise-client-urls":       "https://" + host.InternalAddress + ":2379,https://" + host.InternalAddress + ":4001",
		"listen-client-urls":          "https://0.0.0.0:2379",
		"initial-advertise-peer-urls": "https://" + host.InternalAddress + ":2380",
		"listen-peer-urls":            "https://0.0.0.0:2380",
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
		fmt.Sprintf("%s:/var/lib/rancher/etcd:z", path.Join(prefixPath, "/var/lib/etcd")),
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
		URL: services.EtcdHealthCheckURL,
	}
	registryAuthConfig, _, _ := docker.GetImageRegistryConfig(c.Services.Etcd.Image, c.PrivateRegistriesMap)

	return v3.Process{
		Name:                    services.EtcdContainerName,
		Args:                    args,
		Binds:                   Binds,
		NetworkMode:             "host",
		RestartPolicy:           "always",
		Image:                   c.Services.Etcd.Image,
		HealthCheck:             healthCheck,
		ImageRegistryAuthConfig: registryAuthConfig,
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

func (c *Cluster) getPrefixPath(osType string) string {
	var prefixPath string
	if strings.Contains(osType, B2DOS) {
		prefixPath = B2DPrefixPath
	} else if strings.Contains(osType, ROS) {
		prefixPath = ROSPrefixPath
	} else {
		prefixPath = c.PrefixPath
	}
	return prefixPath
}

func (c *Cluster) GetKubernetesServicesOptions() v3.KubernetesServicesOptions {
	clusterMajorVersion := getTagMajorVersion(c.Version)
	NamedkK8sImage, _ := ref.ParseNormalizedNamed(c.SystemImages.Kubernetes)
	k8sImageTag := NamedkK8sImage.(ref.Tagged).Tag()
	k8sImageMajorVersion := getTagMajorVersion(k8sImageTag)

	if clusterMajorVersion != k8sImageMajorVersion {
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
	return strings.Join(splitTag[:2], ".")
}
