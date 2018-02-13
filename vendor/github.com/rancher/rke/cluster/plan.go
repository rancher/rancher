package cluster

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (v3.RKEPlan, error) {
	clusterPlan := v3.RKEPlan{}
	myCluster, _ := ParseCluster(ctx, rkeConfig, "", "", nil, nil)
	// rkeConfig.Nodes are already unique. But they don't have role flags. So I will use the parsed cluster.Hosts to make use of the role flags.
	uniqHosts := hosts.GetUniqueHostList(myCluster.EtcdHosts, myCluster.ControlPlaneHosts, myCluster.WorkerHosts)
	for _, host := range uniqHosts {
		clusterPlan.Nodes = append(clusterPlan.Nodes, BuildRKEConfigNodePlan(ctx, myCluster, host))
	}
	return clusterPlan, nil
}

func BuildRKEConfigNodePlan(ctx context.Context, myCluster *Cluster, host *hosts.Host) v3.RKEConfigNodePlan {
	processes := []v3.Process{}
	portChecks := []v3.PortCheck{}
	// Everybody gets a sidecar and a kubelet..
	processes = append(processes, myCluster.BuildSidecarProcess())
	processes = append(processes, myCluster.BuildKubeletProcess(host))
	processes = append(processes, myCluster.BuildKubeProxyProcess())

	portChecks = append(portChecks, BuildPortChecksFromPortList(host, WorkerPortList, ProtocolTCP)...)
	// Do we need an nginxProxy for this one ?
	if host.IsWorker && !host.IsControl {
		processes = append(processes, myCluster.BuildProxyProcess())
	}
	if host.IsControl {
		processes = append(processes, myCluster.BuildKubeAPIProcess())
		processes = append(processes, myCluster.BuildKubeControllerProcess())
		processes = append(processes, myCluster.BuildSchedulerProcess())

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, ControlPlanePortList, ProtocolTCP)...)
	}
	if host.IsEtcd {
		processes = append(processes, myCluster.BuildEtcdProcess(host, nil))

		portChecks = append(portChecks, BuildPortChecksFromPortList(host, EtcdPortList, ProtocolTCP)...)
	}
	return v3.RKEConfigNodePlan{
		Address:    host.Address,
		Processes:  processes,
		PortChecks: portChecks,
	}
}

func (c *Cluster) BuildKubeAPIProcess() v3.Process {
	etcdConnString := services.GetEtcdConnString(c.EtcdHosts)
	args := []string{}
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
		"--runtime-config=batch/v2alpha1",
		"--runtime-config=authentication.k8s.io/v1beta1=true",
		"--storage-backend=etcd3",
		"--client-ca-file=" + pki.GetCertPath(pki.CACertName),
		"--tls-cert-file=" + pki.GetCertPath(pki.KubeAPICertName),
		"--tls-private-key-file=" + pki.GetKeyPath(pki.KubeAPICertName),
		"--service-account-key-file=" + pki.GetKeyPath(pki.KubeAPICertName),
		"--etcd-cafile=" + pki.GetCertPath(pki.CACertName),
		"--etcd-certfile=" + pki.GetCertPath(pki.KubeAPICertName),
		"--etcd-keyfile=" + pki.GetKeyPath(pki.KubeAPICertName),
	}
	args = append(args, "--etcd-servers="+etcdConnString)

	if c.Authorization.Mode == services.RBACAuthorizationMode {
		args = append(args, "--authorization-mode=RBAC")
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
		"/etc/resolv.conf:/etc/resolv.conf",
		"/sys:/sys",
		"/var/lib/docker:/var/lib/docker:rw,z",
		"/var/lib/kubelet:/var/lib/kubelet:shared,z",
		"/var/run:/var/run:rw",
		"/run:/run",
		"/etc/ceph:/etc/ceph",
		"/dev:/host/dev",
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
		Env:           Env,
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
		"--data-dir=/etcd-data",
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
		"/var/lib/etcd:/etcd-data:z",
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
		Args:        args,
		Binds:       Binds,
		NetworkMode: "host",
		Image:       c.Services.Etcd.Image,
		HealthCheck: healthCheck,
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
