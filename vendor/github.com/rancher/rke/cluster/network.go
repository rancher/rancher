package cluster

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/templates"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	NetworkPluginResourceName = "rke-network-plugin"

	PortCheckContainer        = "rke-port-checker"
	EtcdPortListenContainer   = "rke-etcd-port-listener"
	CPPortListenContainer     = "rke-cp-port-listener"
	WorkerPortListenContainer = "rke-worker-port-listener"

	KubeAPIPort         = "6443"
	EtcdPort1           = "2379"
	EtcdPort2           = "2380"
	ScedulerPort        = "10251"
	ControllerPort      = "10252"
	KubeletPort         = "10250"
	KubeProxyPort       = "10256"
	FlannetVXLANPortUDP = "8472"

	ProtocolTCP = "TCP"
	ProtocolUDP = "UDP"

	FlannelNetworkPlugin = "flannel"
	FlannelIface         = "flannel_iface"

	CalicoNetworkPlugin = "calico"
	CalicoCloudProvider = "calico_cloud_provider"

	CanalNetworkPlugin = "canal"
	CanalIface         = "canal_iface"

	WeaveNetworkPlugin = "weave"

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
	WeaveLoopbackImage = "WeaveLoopbackImage"

	Calicoctl = "Calicoctl"

	FlannelInterface = "FlannelInterface"
	CanalInterface   = "CanalInterface"
	RBACConfig       = "RBACConfig"
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

func (c *Cluster) deployNetworkPlugin(ctx context.Context) error {
	log.Infof(ctx, "[network] Setting up network plugin: %s", c.Network.Plugin)
	switch c.Network.Plugin {
	case FlannelNetworkPlugin:
		return c.doFlannelDeploy(ctx)
	case CalicoNetworkPlugin:
		return c.doCalicoDeploy(ctx)
	case CanalNetworkPlugin:
		return c.doCanalDeploy(ctx)
	case WeaveNetworkPlugin:
		return c.doWeaveDeploy(ctx)
	default:
		return fmt.Errorf("[network] Unsupported network plugin: %s", c.Network.Plugin)
	}
}

func (c *Cluster) doFlannelDeploy(ctx context.Context) error {
	flannelConfig := map[string]string{
		ClusterCIDR:      c.ClusterCIDR,
		Image:            c.SystemImages.Flannel,
		CNIImage:         c.SystemImages.FlannelCNI,
		FlannelInterface: c.Network.Options[FlannelIface],
		RBACConfig:       c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(flannelConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) doCalicoDeploy(ctx context.Context) error {
	clientConfig := pki.GetConfigPath(pki.KubeNodeCertName)
	calicoConfig := map[string]string{
		KubeCfg:       clientConfig,
		ClusterCIDR:   c.ClusterCIDR,
		CNIImage:      c.SystemImages.CalicoCNI,
		NodeImage:     c.SystemImages.CalicoNode,
		Calicoctl:     c.SystemImages.CalicoCtl,
		CloudProvider: c.Network.Options[CalicoCloudProvider],
		RBACConfig:    c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(calicoConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) doCanalDeploy(ctx context.Context) error {
	clientConfig := pki.GetConfigPath(pki.KubeNodeCertName)
	canalConfig := map[string]string{
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
	}
	pluginYaml, err := c.getNetworkPluginManifest(canalConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) doWeaveDeploy(ctx context.Context) error {
	weaveConfig := map[string]string{
		ClusterCIDR:        c.ClusterCIDR,
		Image:              c.SystemImages.WeaveNode,
		CNIImage:           c.SystemImages.WeaveCNI,
		WeaveLoopbackImage: c.SystemImages.Alpine,
		RBACConfig:         c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(weaveConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName, true)
}

func (c *Cluster) getNetworkPluginManifest(pluginConfig map[string]string) (string, error) {
	switch c.Network.Plugin {
	case FlannelNetworkPlugin:
		return templates.CompileTemplateFromMap(templates.FlannelTemplate, pluginConfig)
	case CalicoNetworkPlugin:
		return templates.CompileTemplateFromMap(templates.CalicoTemplate, pluginConfig)
	case CanalNetworkPlugin:
		return templates.CompileTemplateFromMap(templates.CanalTemplate, pluginConfig)
	case WeaveNetworkPlugin:
		return templates.CompileTemplateFromMap(templates.WeaveTemplate, pluginConfig)
	default:
		return "", fmt.Errorf("[network] Unsupported network plugin: %s", c.Network.Plugin)
	}
}

func (c *Cluster) CheckClusterPorts(ctx context.Context, currentCluster *Cluster) error {
	if currentCluster != nil {
		newEtcdHost := hosts.GetToAddHosts(currentCluster.EtcdHosts, c.EtcdHosts)
		newControlPlanHosts := hosts.GetToAddHosts(currentCluster.ControlPlaneHosts, c.ControlPlaneHosts)
		newWorkerHosts := hosts.GetToAddHosts(currentCluster.WorkerHosts, c.WorkerHosts)

		if len(newEtcdHost) == 0 &&
			len(newWorkerHosts) == 0 &&
			len(newControlPlanHosts) == 0 {
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

	etcdHosts := []*hosts.Host{}
	cpHosts := []*hosts.Host{}
	workerHosts := []*hosts.Host{}
	if currentCluster != nil {
		etcdHosts = hosts.GetToAddHosts(currentCluster.EtcdHosts, c.EtcdHosts)
		cpHosts = hosts.GetToAddHosts(currentCluster.ControlPlaneHosts, c.ControlPlaneHosts)
		workerHosts = hosts.GetToAddHosts(currentCluster.WorkerHosts, c.WorkerHosts)
	} else {
		etcdHosts = c.EtcdHosts
		cpHosts = c.ControlPlaneHosts
		workerHosts = c.WorkerHosts
	}
	// deploy ectd listeners
	if err := c.deployListenerOnPlane(ctx, EtcdPortList, etcdHosts, EtcdPortListenContainer); err != nil {
		return err
	}

	// deploy controlplane listeners
	if err := c.deployListenerOnPlane(ctx, ControlPlanePortList, cpHosts, CPPortListenContainer); err != nil {
		return err
	}

	// deploy worker listeners
	if err := c.deployListenerOnPlane(ctx, WorkerPortList, workerHosts, WorkerPortListenContainer); err != nil {
		return err
	}
	log.Infof(ctx, "[network] Port listener containers deployed successfully")
	return nil
}

func (c *Cluster) deployListenerOnPlane(ctx context.Context, portList []string, holstPlane []*hosts.Host, containerName string) error {
	var errgrp errgroup.Group
	for _, host := range holstPlane {
		runHost := host
		errgrp.Go(func() error {
			return c.deployListener(ctx, runHost, portList, containerName)
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
	for _, host := range hostPlane {
		runHost := host
		errgrp.Go(func() error {
			return docker.DoRemoveContainer(ctx, runHost.DClient, containerName, runHost.Address)
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
		for _, host := range c.EtcdHosts {
			runHost := host
			errgrp.Go(func() error {
				return checkPlaneTCPPortsFromHost(ctx, runHost, EtcdPortList, c.EtcdHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
			})
		}
		if err := errgrp.Wait(); err != nil {
			return err
		}
	}
	// check all -> etcd connectivity
	log.Infof(ctx, "[network] Running control plane -> etcd port checks")
	for _, host := range c.ControlPlaneHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, EtcdPortList, c.EtcdHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	// check controle plane -> Workers
	log.Infof(ctx, "[network] Running control plane -> worker port checks")
	for _, host := range c.ControlPlaneHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, WorkerPortList, c.WorkerHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	// check workers -> control plane
	log.Infof(ctx, "[network] Running workers -> control plane port checks")
	for _, host := range c.WorkerHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, ControlPlanePortList, c.ControlPlaneHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
		})
	}
	return errgrp.Wait()
}

func checkPlaneTCPPortsFromHost(ctx context.Context, host *hosts.Host, portList []string, planeHosts []*hosts.Host, image string, prsMap map[string]v3.PrivateRegistry) error {
	var hosts []string
	var containerStdout bytes.Buffer
	var containerStderr bytes.Buffer

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

	clogs, err := docker.ReadContainerLogs(ctx, host.DClient, PortCheckContainer, true, "all")
	if err != nil {
		return err
	}
	defer clogs.Close()

	stdcopy.StdCopy(&containerStdout, &containerStderr, clogs)
	containerLog := containerStderr.String()
	logrus.Debugf("[network] containerLog [%s] on host: %s", containerLog, host.Address)

	if err := docker.RemoveContainer(ctx, host.DClient, host.Address, PortCheckContainer); err != nil {
		return err
	}
	logrus.Debugf("[network] Length of containerLog is [%d] on host: %s", len(containerLog), host.Address)
	if len(containerLog) > 0 {
		portCheckLogs := strings.Join(strings.Split(strings.TrimSpace(containerLog), "\n"), ", ")
		return fmt.Errorf("[network] Port check for ports: [%s] failed on host: [%s]", portCheckLogs, host.Address)
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
