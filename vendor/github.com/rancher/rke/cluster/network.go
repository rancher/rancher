package cluster

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/templates"
	"github.com/rancher/rke/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	NetworkPluginResourceName = "rke-network-plugin"

	PortCheckContainer        = "rke-port-checker"
	EtcdPortListenContainer   = "rke-etcd-port-listener"
	CPPortListenContainer     = "rke-cp-port-listener"
	WorkerPortListenContainer = "rke-worker-port-listener"

	KubeAPIPort      = "6443"
	EtcdPort1        = "2379"
	EtcdPort2        = "2380"
	ScedulerPort     = "10251"
	ControllerPort   = "10252"
	KubeletPort      = "10250"
	KubeProxyPort    = "10256"
	FlannelVxLanPort = 8472

	FlannelVxLanNetworkIdentify = 1

	ProtocolTCP = "TCP"
	ProtocolUDP = "UDP"

	NoNetworkPlugin = "none"

	FlannelNetworkPlugin = "flannel"
	FlannelIface         = "flannel_iface"
	FlannelBackendType   = "flannel_backend_type"
	// FlannelBackendPort must be 4789 if using VxLan mode in the cluster with Windows nodes
	FlannelBackendPort = "flannel_backend_port"
	// FlannelBackendVxLanNetworkIdentify should be greater than or equal to 4096 if using VxLan mode in the cluster with Windows nodes
	FlannelBackendVxLanNetworkIdentify = "flannel_backend_vni"

	CalicoNetworkPlugin          = "calico"
	CalicoNodeLabel              = "calico-node"
	CalicoControllerLabel        = "calico-kube-controllers"
	CalicoCloudProvider          = "calico_cloud_provider"
	CalicoFlexVolPluginDirectory = "calico_flex_volume_plugin_dir"

	CanalNetworkPlugin      = "canal"
	CanalIface              = "canal_iface"
	CanalFlannelBackendType = "canal_flannel_backend_type"
	// CanalFlannelBackendPort must be 4789 if using Flannel VxLan mode in the cluster with Windows nodes
	CanalFlannelBackendPort = "canal_flannel_backend_port"
	// CanalFlannelBackendVxLanNetworkIdentify should be greater than or equal to 4096 if using Flannel VxLan mode in the cluster with Windows nodes
	CanalFlannelBackendVxLanNetworkIdentify = "canal_flannel_backend_vni"
	CanalFlexVolPluginDirectory             = "canal_flex_volume_plugin_dir"

	WeaveNetworkPlugin  = "weave"
	WeaveNetworkAppName = "weave-net"
	// List of map keys to be used with network templates

	// EtcdEndpoints is the server address for Etcd, used by calico
	EtcdEndpoints = "EtcdEndpoints"
	// APIRoot is the kubernetes API address
	APIRoot = "APIRoot"
	// kubernetes client certificates and kubeconfig paths

	EtcdClientCert     = "EtcdClientCert"
	EtcdClientKey      = "EtcdClientKey"
	EtcdClientCA       = "EtcdClientCA"
	EtcdClientCertPath = "EtcdClientCertPath"
	EtcdClientKeyPath  = "EtcdClientKeyPath"
	EtcdClientCAPath   = "EtcdClientCAPath"

	ClientCertPath = "ClientCertPath"
	ClientKeyPath  = "ClientKeyPath"
	ClientCAPath   = "ClientCAPath"

	KubeCfg = "KubeCfg"

	ClusterCIDR = "ClusterCIDR"
	// Images key names

	Image              = "Image"
	CNIImage           = "CNIImage"
	NodeImage          = "NodeImage"
	ControllersImage   = "ControllersImage"
	CanalFlannelImg    = "CanalFlannelImg"
	FlexVolImg         = "FlexVolImg"
	WeaveLoopbackImage = "WeaveLoopbackImage"

	Calicoctl = "Calicoctl"

	FlannelInterface = "FlannelInterface"
	FlannelBackend   = "FlannelBackend"
	CanalInterface   = "CanalInterface"
	FlexVolPluginDir = "FlexVolPluginDir"
	WeavePassword    = "WeavePassword"
	MTU              = "MTU"
	RBACConfig       = "RBACConfig"
	ClusterVersion   = "ClusterVersion"

	NodeSelector   = "NodeSelector"
	UpdateStrategy = "UpdateStrategy"
)

var EtcdPortList = []string{
	EtcdPort1,
	EtcdPort2,
}

var ControlPlanePortList = []string{
	KubeAPIPort,
}

var WorkerPortList = []string{
	KubeletPort,
}

var EtcdClientPortList = []string{
	EtcdPort1,
}

var CalicoNetworkLabels = []string{CalicoNodeLabel, CalicoControllerLabel}

func (c *Cluster) deployNetworkPlugin(ctx context.Context, data map[string]interface{}) error {
	log.Infof(ctx, "[network] Setting up network plugin: %s", c.Network.Plugin)
	switch c.Network.Plugin {
	case FlannelNetworkPlugin:
		return c.doFlannelDeploy(ctx, data)
	case CalicoNetworkPlugin:
		return c.doCalicoDeploy(ctx, data)
	case CanalNetworkPlugin:
		return c.doCanalDeploy(ctx, data)
	case WeaveNetworkPlugin:
		return c.doWeaveDeploy(ctx, data)
	case NoNetworkPlugin:
		log.Infof(ctx, "[network] Not deploying a cluster network, expecting custom CNI")
		return nil
	default:
		return fmt.Errorf("[network] Unsupported network plugin: %s", c.Network.Plugin)
	}
}

func (c *Cluster) doFlannelDeploy(ctx context.Context, data map[string]interface{}) error {
	vni, err := atoiWithDefault(c.Network.Options[FlannelBackendVxLanNetworkIdentify], FlannelVxLanNetworkIdentify)
	if err != nil {
		return err
	}
	port, err := atoiWithDefault(c.Network.Options[FlannelBackendPort], FlannelVxLanPort)
	if err != nil {
		return err
	}

	flannelConfig := map[string]interface{}{
		ClusterCIDR:      c.ClusterCIDR,
		Image:            c.SystemImages.Flannel,
		CNIImage:         c.SystemImages.FlannelCNI,
		FlannelInterface: c.Network.Options[FlannelIface],
		FlannelBackend: map[string]interface{}{
			"Type": c.Network.Options[FlannelBackendType],
			"VNI":  vni,
			"Port": port,
		},
		RBACConfig:     c.Authorization.Mode,
		ClusterVersion: util.GetTagMajorVersion(c.Version),
		NodeSelector:   c.Network.NodeSelector,
		UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
			Type:          c.Network.UpdateStrategy.Strategy,
			RollingUpdate: c.Network.UpdateStrategy.RollingUpdate,
		},
	}
	pluginYaml, err := c.getNetworkPluginManifest(flannelConfig, data)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) doCalicoDeploy(ctx context.Context, data map[string]interface{}) error {
	clientConfig := pki.GetConfigPath(pki.KubeNodeCertName)

	calicoConfig := map[string]interface{}{
		KubeCfg:          clientConfig,
		ClusterCIDR:      c.ClusterCIDR,
		CNIImage:         c.SystemImages.CalicoCNI,
		NodeImage:        c.SystemImages.CalicoNode,
		Calicoctl:        c.SystemImages.CalicoCtl,
		ControllersImage: c.SystemImages.CalicoControllers,
		CloudProvider:    c.Network.Options[CalicoCloudProvider],
		FlexVolImg:       c.SystemImages.CalicoFlexVol,
		RBACConfig:       c.Authorization.Mode,
		NodeSelector:     c.Network.NodeSelector,
		MTU:              c.Network.MTU,
		UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
			Type:          c.Network.UpdateStrategy.Strategy,
			RollingUpdate: c.Network.UpdateStrategy.RollingUpdate,
		},
		FlexVolPluginDir: c.Network.Options[CalicoFlexVolPluginDirectory],
	}
	pluginYaml, err := c.getNetworkPluginManifest(calicoConfig, data)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) doCanalDeploy(ctx context.Context, data map[string]interface{}) error {
	flannelVni, err := atoiWithDefault(c.Network.Options[CanalFlannelBackendVxLanNetworkIdentify], FlannelVxLanNetworkIdentify)
	if err != nil {
		return err
	}
	flannelPort, err := atoiWithDefault(c.Network.Options[CanalFlannelBackendPort], FlannelVxLanPort)
	if err != nil {
		return err
	}

	clientConfig := pki.GetConfigPath(pki.KubeNodeCertName)
	canalConfig := map[string]interface{}{
		ClientCertPath:  pki.GetCertPath(pki.KubeNodeCertName),
		APIRoot:         "https://127.0.0.1:6443",
		ClientKeyPath:   pki.GetKeyPath(pki.KubeNodeCertName),
		ClientCAPath:    pki.GetCertPath(pki.CACertName),
		KubeCfg:         clientConfig,
		ClusterCIDR:     c.ClusterCIDR,
		NodeImage:       c.SystemImages.CanalNode,
		CNIImage:        c.SystemImages.CanalCNI,
		CanalFlannelImg: c.SystemImages.CanalFlannel,
		RBACConfig:      c.Authorization.Mode,
		CanalInterface:  c.Network.Options[CanalIface],
		FlexVolImg:      c.SystemImages.CanalFlexVol,
		FlannelBackend: map[string]interface{}{
			"Type": c.Network.Options[CanalFlannelBackendType],
			"VNI":  flannelVni,
			"Port": flannelPort,
		},
		NodeSelector: c.Network.NodeSelector,
		MTU:          c.Network.MTU,
		UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
			Type:          c.Network.UpdateStrategy.Strategy,
			RollingUpdate: c.Network.UpdateStrategy.RollingUpdate,
		},
		FlexVolPluginDir: c.Network.Options[CanalFlexVolPluginDirectory],
	}
	pluginYaml, err := c.getNetworkPluginManifest(canalConfig, data)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) doWeaveDeploy(ctx context.Context, data map[string]interface{}) error {
	weaveConfig := map[string]interface{}{
		ClusterCIDR:        c.ClusterCIDR,
		WeavePassword:      c.Network.Options[WeavePassword],
		Image:              c.SystemImages.WeaveNode,
		CNIImage:           c.SystemImages.WeaveCNI,
		WeaveLoopbackImage: c.SystemImages.Alpine,
		RBACConfig:         c.Authorization.Mode,
		NodeSelector:       c.Network.NodeSelector,
		MTU:                c.Network.MTU,
		UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
			Type:          c.Network.UpdateStrategy.Strategy,
			RollingUpdate: c.Network.UpdateStrategy.RollingUpdate,
		},
	}
	pluginYaml, err := c.getNetworkPluginManifest(weaveConfig, data)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) getNetworkPluginManifest(pluginConfig, data map[string]interface{}) (string, error) {
	switch c.Network.Plugin {
	case CanalNetworkPlugin, FlannelNetworkPlugin, CalicoNetworkPlugin, WeaveNetworkPlugin:
		tmplt, err := templates.GetVersionedTemplates(c.Network.Plugin, data, c.Version)
		if err != nil {
			return "", err
		}
		return templates.CompileTemplateFromMap(tmplt, pluginConfig)
	default:
		return "", fmt.Errorf("[network] Unsupported network plugin: %s", c.Network.Plugin)
	}
}

func (c *Cluster) CheckClusterPorts(ctx context.Context, currentCluster *Cluster) error {
	if currentCluster != nil {
		newEtcdHost := hosts.GetToAddHosts(currentCluster.EtcdHosts, c.EtcdHosts)
		newControlPlaneHosts := hosts.GetToAddHosts(currentCluster.ControlPlaneHosts, c.ControlPlaneHosts)
		newWorkerHosts := hosts.GetToAddHosts(currentCluster.WorkerHosts, c.WorkerHosts)

		if len(newEtcdHost) == 0 &&
			len(newWorkerHosts) == 0 &&
			len(newControlPlaneHosts) == 0 {
			log.Infof(ctx, "[network] No hosts added existing cluster, skipping port check")
			return nil
		}
	}
	if err := c.deployTCPPortListeners(ctx, currentCluster); err != nil {
		return err
	}
	if err := c.runServicePortChecks(ctx); err != nil {
		return err
	}
	// Skip kubeapi check if we are using custom k8s dialer or bastion/jump host
	if c.K8sWrapTransport == nil && len(c.BastionHost.Address) == 0 {
		if err := c.checkKubeAPIPort(ctx); err != nil {
			return err
		}
	} else {
		log.Infof(ctx, "[network] Skipping kubeapi port check")
	}

	return c.removeTCPPortListeners(ctx)
}

func (c *Cluster) checkKubeAPIPort(ctx context.Context) error {
	log.Infof(ctx, "[network] Checking KubeAPI port Control Plane hosts")
	for _, host := range c.ControlPlaneHosts {
		logrus.Debugf("[network] Checking KubeAPI port [%s] on host: %s", KubeAPIPort, host.Address)
		address := fmt.Sprintf("%s:%s", host.Address, KubeAPIPort)
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("[network] Can't access KubeAPI port [%s] on Control Plane host: %s", KubeAPIPort, host.Address)
		}
		conn.Close()
	}
	return nil
}

func (c *Cluster) deployTCPPortListeners(ctx context.Context, currentCluster *Cluster) error {
	log.Infof(ctx, "[network] Deploying port listener containers")

	// deploy ectd listeners
	if err := c.deployListenerOnPlane(ctx, EtcdPortList, c.EtcdHosts, EtcdPortListenContainer); err != nil {
		return err
	}

	// deploy controlplane listeners
	if err := c.deployListenerOnPlane(ctx, ControlPlanePortList, c.ControlPlaneHosts, CPPortListenContainer); err != nil {
		return err
	}

	// deploy worker listeners
	if err := c.deployListenerOnPlane(ctx, WorkerPortList, c.WorkerHosts, WorkerPortListenContainer); err != nil {
		return err
	}
	log.Infof(ctx, "[network] Port listener containers deployed successfully")
	return nil
}

func (c *Cluster) deployListenerOnPlane(ctx context.Context, portList []string, hostPlane []*hosts.Host, containerName string) error {
	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(hostPlane)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := c.deployListener(ctx, host.(*hosts.Host), portList, containerName)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	return errgrp.Wait()
}

func (c *Cluster) deployListener(ctx context.Context, host *hosts.Host, portList []string, containerName string) error {
	imageCfg := &container.Config{
		Image: c.SystemImages.Alpine,
		Cmd: []string{
			"nc",
			"-kl",
			"-p",
			"1337",
			"-e",
			"echo",
		},
		ExposedPorts: nat.PortSet{
			"1337/tcp": {},
		},
	}
	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			"1337/tcp": getPortBindings("0.0.0.0", portList),
		},
	}

	logrus.Debugf("[network] Starting deployListener [%s] on host [%s]", containerName, host.Address)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, containerName, host.Address, "network", c.PrivateRegistriesMap); err != nil {
		if strings.Contains(err.Error(), "bind: address already in use") {
			logrus.Debugf("[network] Service is already up on host [%s]", host.Address)
			return nil
		}
		return err
	}
	return nil
}

func (c *Cluster) removeTCPPortListeners(ctx context.Context) error {
	log.Infof(ctx, "[network] Removing port listener containers")

	if err := removeListenerFromPlane(ctx, c.EtcdHosts, EtcdPortListenContainer); err != nil {
		return err
	}
	if err := removeListenerFromPlane(ctx, c.ControlPlaneHosts, CPPortListenContainer); err != nil {
		return err
	}
	if err := removeListenerFromPlane(ctx, c.WorkerHosts, WorkerPortListenContainer); err != nil {
		return err
	}
	log.Infof(ctx, "[network] Port listener containers removed successfully")
	return nil
}

func removeListenerFromPlane(ctx context.Context, hostPlane []*hosts.Host, containerName string) error {
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(hostPlane)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				err := docker.DoRemoveContainer(ctx, runHost.DClient, containerName, runHost.Address)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	return errgrp.Wait()
}

func (c *Cluster) runServicePortChecks(ctx context.Context) error {
	var errgrp errgroup.Group
	// check etcd <-> etcd
	// one etcd host is a pass
	if len(c.EtcdHosts) > 1 {
		log.Infof(ctx, "[network] Running etcd <-> etcd port checks")
		hostsQueue := util.GetObjectQueue(c.EtcdHosts)
		for w := 0; w < WorkerThreads; w++ {
			errgrp.Go(func() error {
				var errList []error
				for host := range hostsQueue {
					err := checkPlaneTCPPortsFromHost(ctx, host.(*hosts.Host), EtcdPortList, c.EtcdHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
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
	}
	// check control -> etcd connectivity
	log.Infof(ctx, "[network] Running control plane -> etcd port checks")
	hostsQueue := util.GetObjectQueue(c.ControlPlaneHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := checkPlaneTCPPortsFromHost(ctx, host.(*hosts.Host), EtcdClientPortList, c.EtcdHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
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
	// check controle plane -> Workers
	log.Infof(ctx, "[network] Running control plane -> worker port checks")
	hostsQueue = util.GetObjectQueue(c.ControlPlaneHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := checkPlaneTCPPortsFromHost(ctx, host.(*hosts.Host), WorkerPortList, c.WorkerHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
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
	// check workers -> control plane
	log.Infof(ctx, "[network] Running workers -> control plane port checks")
	hostsQueue = util.GetObjectQueue(c.WorkerHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := checkPlaneTCPPortsFromHost(ctx, host.(*hosts.Host), ControlPlanePortList, c.ControlPlaneHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	return errgrp.Wait()
}

func checkPlaneTCPPortsFromHost(ctx context.Context, host *hosts.Host, portList []string, planeHosts []*hosts.Host, image string, prsMap map[string]v3.PrivateRegistry) error {
	var hosts []string

	for _, host := range planeHosts {
		hosts = append(hosts, host.InternalAddress)
	}
	imageCfg := &container.Config{
		Image: image,
		Env: []string{
			fmt.Sprintf("HOSTS=%s", strings.Join(hosts, " ")),
			fmt.Sprintf("PORTS=%s", strings.Join(portList, " ")),
		},
		Cmd: []string{
			"sh",
			"-c",
			"for host in $HOSTS; do for port in $PORTS ; do echo \"Checking host ${host} on port ${port}\" >&1 & nc -w 5 -z $host $port > /dev/null || echo \"${host}:${port}\" >&2 & done; wait; done",
		},
	}
	hostCfg := &container.HostConfig{
		NetworkMode: "host",
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, PortCheckContainer, host.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, PortCheckContainer, host.Address, "network", prsMap); err != nil {
		return err
	}

	containerLog, _, logsErr := docker.GetContainerLogsStdoutStderr(ctx, host.DClient, PortCheckContainer, "all", true)
	if logsErr != nil {
		log.Warnf(ctx, "[network] Failed to get network port check logs: %v", logsErr)
	}
	logrus.Debugf("[network] containerLog [%s] on host: %s", containerLog, host.Address)

	if err := docker.RemoveContainer(ctx, host.DClient, host.Address, PortCheckContainer); err != nil {
		return err
	}
	logrus.Debugf("[network] Length of containerLog is [%d] on host: %s", len(containerLog), host.Address)
	if len(containerLog) > 0 {
		portCheckLogs := strings.Join(strings.Split(strings.TrimSpace(containerLog), "\n"), ", ")
		return fmt.Errorf("[network] Host [%s] is not able to connect to the following ports: [%s]. Please check network policies and firewall rules", host.Address, portCheckLogs)
	}
	return nil
}

func getPortBindings(hostAddress string, portList []string) []nat.PortBinding {
	portBindingList := []nat.PortBinding{}
	for _, portNumber := range portList {
		rawPort := fmt.Sprintf("%s:%s:1337/tcp", hostAddress, portNumber)
		portMapping, _ := nat.ParsePortSpec(rawPort)
		portBindingList = append(portBindingList, portMapping[0].Binding)
	}
	return portBindingList
}

func atoiWithDefault(val string, defaultVal int) (int, error) {
	if val == "" {
		return defaultVal, nil
	}

	ret, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}

	return ret, nil
}
