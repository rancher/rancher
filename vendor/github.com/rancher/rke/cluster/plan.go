package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	EtcdPathPrefix = "/registry"
)

func GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (v3.RKEPlan, error) {
	clusterPlan := v3.RKEPlan{}
	myCluster, _ := ParseCluster(ctx, rkeConfig, "", "", nil, nil, nil)
	// rkeConfig.Nodes are already unique. But they don't have role flags. So I will use the parsed cluster.Hosts to make use of the role flags.
	uniqHosts := hosts.GetUniqueHostList(myCluster.EtcdHosts, myCluster.ControlPlaneHosts, myCluster.WorkerHosts)
	for _, host := range uniqHosts {
		clusterPlan.Nodes = append(clusterPlan.Nodes, BuildRKEConfigNodePlan(ctx, myCluster, host))
	}
	return clusterPlan, nil
}

func BuildRKEConfigNodePlan(ctx context.Context, myCluster *Cluster, host *hosts.Host) v3.RKEConfigNodePlan {
	processes := map[string]v3.Process{}
	portChecks := []v3.PortCheck{}
	// Everybody gets a sidecar and a kubelet..
	processes[services.SidekickContainerName] = myCluster.BuildSidecarProcess()
	processes[services.KubeletContainerName] = myCluster.BuildKubeletProcess(host)
	processes[services.KubeproxyContainerName] = myCluster.BuildKubeProxyProcess()

	portChecks = append(portChecks, BuildPortChecksFromPortList(host, WorkerPortList, ProtocolTCP)...)
	// Do we need an nginxProxy for this one ?
	if host.IsWorker && !host.IsControl {
		processes[services.NginxProxyContainerName] = myCluster.BuildProxyProcess()
	}
	if host.IsControl {
		processes[services.KubeAPIContainerName] = myCluster.BuildKubeAPIProcess()
		processes[services.KubeControllerContainerName] = myCluster.BuildKubeControllerProcess()
		processes[services.SchedulerContainerName] = myCluster.BuildSchedulerProcess()

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, ControlPlanePortList, ProtocolTCP)...)
	}
	if host.IsEtcd {
		processes[services.EtcdContainerName] = myCluster.BuildEtcdProcess(host, nil)

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, EtcdPortList, ProtocolTCP)...)
	}
	return v3.RKEConfigNodePlan{
		Address:    host.Address,
		Processes:  processes,
		PortChecks: portChecks,
	}
}

func (c *Cluster) BuildKubeAPIProcess() v3.Process {
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
		"--insecure-bind-address=127.0.0.1",
		"--bind-address=0.0.0.0",
		"--insecure-port=0",
		"--secure-port=6443",
		"--cloud-provider=",
		"--allow_privileged=true",
		"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
		"--service-cluster-ip-range=" + c.Services.KubeAPI.ServiceClusterIPRange,
		"--admission-control=ServiceAccount,NamespaceLifecycle,LimitRanger,PersistentVolumeLabel,DefaultStorageClass,ResourceQuota,DefaultTolerationSeconds",
		"--storage-backend=etcd3",
		"--client-ca-file=" + pki.GetCertPath(pki.CACertName),
		"--tls-cert-file=" + pki.GetCertPath(pki.KubeAPICertName),
		"--tls-private-key-file=" + pki.GetKeyPath(pki.KubeAPICertName),
		"--kubelet-client-certificate=" + pki.GetCertPath(pki.KubeAPICertName),
		"--kubelet-client-key=" + pki.GetKeyPath(pki.KubeAPICertName),
		"--service-account-key-file=" + pki.GetKeyPath(pki.KubeAPICertName),
	}
	args := []string{
		"--etcd-cafile=" + etcdCAClientCert,
		"--etcd-certfile=" + etcdClientCert,
		"--etcd-keyfile=" + etcdClientKey,
		"--etcd-servers=" + etcdConnectionString,
		"--etcd-prefix=" + etcdPathPrefix,
	}

	if c.Authorization.Mode == services.RBACAuthorizationMode {
		args = append(args, "--authorization-mode=Node,RBAC")
	}
	if c.Services.KubeAPI.PodSecurityPolicy {
		args = append(args, "--runtime-config=extensions/v1beta1/podsecuritypolicy=true", "--admission-control=PodSecurityPolicy")
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		"/etc/kubernetes:/etc/kubernetes:z",
	}

	for arg, value := range c.Services.KubeAPI.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}
	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(true, services.KubeAPIPort),
	}
	return v3.Process{
		Name:          services.KubeAPIContainerName,
		Command:       Command,
		Args:          args,
		VolumesFrom:   VolumesFrom,
		Binds:         Binds,
		NetworkMode:   "host",
		RestartPolicy: "always",
		Image:         c.Services.KubeAPI.Image,
		HealthCheck:   healthCheck,
	}
}

func (c *Cluster) BuildKubeControllerProcess() v3.Process {
	Command := []string{"/opt/rke/entrypoint.sh",
		"kube-controller-manager",
		"--address=0.0.0.0",
		"--cloud-provider=",
		"--leader-elect=true",
		"--kubeconfig=" + pki.GetConfigPath(pki.KubeControllerCertName),
		"--enable-hostpath-provisioner=false",
		"--node-monitor-grace-period=40s",
		"--pod-eviction-timeout=5m0s",
		"--v=2",
		"--allocate-node-cidrs=true",
		"--cluster-cidr=" + c.ClusterCIDR,
		"--service-cluster-ip-range=" + c.Services.KubeController.ServiceClusterIPRange,
		"--service-account-private-key-file=" + pki.GetKeyPath(pki.KubeAPICertName),
		"--root-ca-file=" + pki.GetCertPath(pki.CACertName),
	}
	args := []string{}
	if c.Authorization.Mode == services.RBACAuthorizationMode {
		args = append(args, "--use-service-account-credentials=true")
	}
	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		"/etc/kubernetes:/etc/kubernetes:z",
	}

	for arg, value := range c.Services.KubeController.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}
	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.KubeControllerPort),
	}
	return v3.Process{
		Name:          services.KubeControllerContainerName,
		Command:       Command,
		Args:          args,
		VolumesFrom:   VolumesFrom,
		Binds:         Binds,
		NetworkMode:   "host",
		RestartPolicy: "always",
		Image:         c.Services.KubeController.Image,
		HealthCheck:   healthCheck,
	}
}

func (c *Cluster) BuildKubeletProcess(host *hosts.Host) v3.Process {

	Command := []string{"/opt/rke/entrypoint.sh",
		"kubelet",
		"--v=2",
		"--address=0.0.0.0",
		"--cadvisor-port=0",
		"--read-only-port=0",
		"--cluster-domain=" + c.ClusterDomain,
		"--pod-infra-container-image=" + c.Services.Kubelet.InfraContainerImage,
		"--cgroups-per-qos=True",
		"--enforce-node-allocatable=",
		"--hostname-override=" + host.HostnameOverride,
		"--cluster-dns=" + c.ClusterDNSServer,
		"--network-plugin=cni",
		"--cni-conf-dir=/etc/cni/net.d",
		"--cni-bin-dir=/opt/cni/bin",
		"--resolv-conf=/etc/resolv.conf",
		"--allow-privileged=true",
		"--cloud-provider=",
		"--kubeconfig=" + pki.GetConfigPath(pki.KubeNodeCertName),
		"--client-ca-file=" + pki.GetCertPath(pki.CACertName),
		"--anonymous-auth=false",
		"--volume-plugin-dir=/var/lib/kubelet/volumeplugins",
		"--require-kubeconfig=True",
		"--fail-swap-on=" + strconv.FormatBool(c.Services.Kubelet.FailSwapOn),
	}

	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		"/etc/kubernetes:/etc/kubernetes:z",
		"/etc/cni:/etc/cni:ro,z",
		"/opt/cni:/opt/cni:ro,z",
		"/var/lib/cni:/var/lib/cni:z",
		"/etc/resolv.conf:/etc/resolv.conf",
		"/sys:/sys:rprivate",
		host.DockerInfo.DockerRootDir + ":" + host.DockerInfo.DockerRootDir + ":rw,rprivate,z",
		"/var/lib/kubelet:/var/lib/kubelet:shared,z",
		"/var/run:/var/run:rw,rprivate",
		"/run:/run:rprivate",
		"/etc/ceph:/etc/ceph",
		"/dev:/host/dev,rprivate",
		"/var/log/containers:/var/log/containers:z",
		"/var/log/pods:/var/log/pods:z",
	}

	for arg, value := range c.Services.Kubelet.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}
	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(true, services.KubeletPort),
	}
	return v3.Process{
		Name:          services.KubeletContainerName,
		Command:       Command,
		VolumesFrom:   VolumesFrom,
		Binds:         Binds,
		NetworkMode:   "host",
		RestartPolicy: "always",
		Image:         c.Services.Kubelet.Image,
		PidMode:       "host",
		Privileged:    true,
		HealthCheck:   healthCheck,
	}
}

func (c *Cluster) BuildKubeProxyProcess() v3.Process {
	Command := []string{"/opt/rke/entrypoint.sh",
		"kube-proxy",
		"--v=2",
		"--healthz-bind-address=0.0.0.0",
		"--kubeconfig=" + pki.GetConfigPath(pki.KubeProxyCertName),
	}
	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		"/etc/kubernetes:/etc/kubernetes:z",
	}

	for arg, value := range c.Services.Kubeproxy.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}
	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.KubeproxyPort),
	}
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

	return v3.Process{
		Name:          services.NginxProxyContainerName,
		Env:           Env,
		Args:          Env,
		NetworkMode:   "host",
		RestartPolicy: "always",
		HealthCheck:   v3.HealthCheck{},
		Image:         c.SystemImages.NginxProxy,
	}
}

func (c *Cluster) BuildSchedulerProcess() v3.Process {
	Command := []string{"/opt/rke/entrypoint.sh",
		"kube-scheduler",
		"--leader-elect=true",
		"--v=2",
		"--address=0.0.0.0",
		"--kubeconfig=" + pki.GetConfigPath(pki.KubeSchedulerCertName),
	}
	VolumesFrom := []string{
		services.SidekickContainerName,
	}
	Binds := []string{
		"/etc/kubernetes:/etc/kubernetes:z",
	}

	for arg, value := range c.Services.Scheduler.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		Command = append(Command, cmd)
	}
	healthCheck := v3.HealthCheck{
		URL: services.GetHealthCheckURL(false, services.SchedulerPort),
	}
	return v3.Process{
		Name:          services.SchedulerContainerName,
		Command:       Command,
		Binds:         Binds,
		VolumesFrom:   VolumesFrom,
		NetworkMode:   "host",
		RestartPolicy: "always",
		Image:         c.Services.Scheduler.Image,
		HealthCheck:   healthCheck,
	}
}

func (c *Cluster) BuildSidecarProcess() v3.Process {
	return v3.Process{
		Name:        services.SidekickContainerName,
		NetworkMode: "none",
		Image:       c.SystemImages.KubernetesServicesSidecar,
		HealthCheck: v3.HealthCheck{},
	}
}

func (c *Cluster) BuildEtcdProcess(host *hosts.Host, etcdHosts []*hosts.Host) v3.Process {
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
	args := []string{"/usr/local/bin/etcd",
		"--name=etcd-" + host.HostnameOverride,
		"--data-dir=/var/lib/rancher/etcd",
		"--advertise-client-urls=https://" + host.InternalAddress + ":2379,https://" + host.InternalAddress + ":4001",
		"--listen-client-urls=https://0.0.0.0:2379",
		"--initial-advertise-peer-urls=https://" + host.InternalAddress + ":2380",
		"--listen-peer-urls=https://0.0.0.0:2380",
		"--initial-cluster-token=etcd-cluster-1",
		"--initial-cluster=" + initCluster,
		"--initial-cluster-state=" + clusterState,
		"--peer-client-cert-auth",
		"--client-cert-auth",
		"--trusted-ca-file=" + pki.GetCertPath(pki.CACertName),
		"--peer-trusted-ca-file=" + pki.GetCertPath(pki.CACertName),
		"--cert-file=" + pki.GetCertPath(nodeName),
		"--key-file=" + pki.GetKeyPath(nodeName),
		"--peer-cert-file=" + pki.GetCertPath(nodeName),
		"--peer-key-file=" + pki.GetKeyPath(nodeName),
	}

	Binds := []string{
		"/var/lib/etcd:/var/lib/rancher/etcd:z",
		"/etc/kubernetes:/etc/kubernetes:z",
	}
	for arg, value := range c.Services.Etcd.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		args = append(args, cmd)
	}
	healthCheck := v3.HealthCheck{
		URL: services.EtcdHealthCheckURL,
	}
	return v3.Process{
		Name:          services.EtcdContainerName,
		Args:          args,
		Binds:         Binds,
		NetworkMode:   "host",
		RestartPolicy: "always",
		Image:         c.Services.Etcd.Image,
		HealthCheck:   healthCheck,
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
