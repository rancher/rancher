package cluster

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	b64 "encoding/base64"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/templates"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/util/cert"
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

	FlannelNetworkPlugin = "flannel"
	FlannelImage         = "flannel_image"
	FlannelCNIImage      = "flannel_cni_image"
	FlannelIface         = "flannel_iface"

	CalicoNetworkPlugin    = "calico"
	CalicoNodeImage        = "calico_node_image"
	CalicoCNIImage         = "calico_cni_image"
	CalicoControllersImage = "calico_controllers_image"
	CalicoctlImage         = "calicoctl_image"
	CalicoCloudProvider    = "calico_cloud_provider"

	CanalNetworkPlugin = "canal"
	CanalNodeImage     = "canal_node_image"
	CanalCNIImage      = "canal_cni_image"
	CanalFlannelImage  = "canal_flannel_image"

	WeaveNetworkPlugin = "weave"
	WeaveImage         = "weave_node_image"
	WeaveCNIImage      = "weave_cni_image"

	// List of map keys to be used with network templates

	// EtcdEndpoints is the server address for Etcd, used by calico
	EtcdEndpoints = "EtcdEndpoints"
	// APIRoot is the kubernetes API address
	APIRoot = "APIRoot"
	// kubernetes client certificates and kubeconfig paths

	ClientCert     = "ClientCert"
	ClientCertPath = "ClientCertPath"
	ClientKey      = "ClientKey"
	ClientKeyPath  = "ClientKeyPath"
	ClientCA       = "ClientCA"
	ClientCAPath   = "ClientCAPath"

	KubeCfg = "KubeCfg"

	ClusterCIDR = "ClusterCIDR"
	// Images key names

	Image            = "Image"
	CNIImage         = "CNIImage"
	NodeImage        = "NodeImage"
	ControllersImage = "ControllersImage"
	CanalFlannelImg  = "CanalFlannelImg"

	Calicoctl = "Calicoctl"

	FlannelInterface = "FlannelInterface"
	CloudProvider    = "CloudProvider"
	AWSCloudProvider = "aws"
	RBACConfig       = "RBACConfig"
)

func (c *Cluster) DeployNetworkPlugin(ctx context.Context) error {
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
		Image:            c.Network.Options[FlannelImage],
		CNIImage:         c.Network.Options[FlannelCNIImage],
		FlannelInterface: c.Network.Options[FlannelIface],
		RBACConfig:       c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(flannelConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCalicoDeploy(ctx context.Context) error {
	clientCert := b64.StdEncoding.EncodeToString(cert.EncodeCertPEM(c.Certificates[pki.KubeNodeCertName].Certificate))
	clientkey := b64.StdEncoding.EncodeToString(cert.EncodePrivateKeyPEM(c.Certificates[pki.KubeNodeCertName].Key))
	clientConfig := pki.GetConfigPath(pki.KubeNodeCertName)
	caCert := b64.StdEncoding.EncodeToString(cert.EncodeCertPEM(c.Certificates[pki.CACertName].Certificate))
	calicoConfig := map[string]string{
		EtcdEndpoints:    services.GetEtcdConnString(c.EtcdHosts),
		APIRoot:          "https://127.0.0.1:6443",
		ClientCert:       clientCert,
		ClientCertPath:   pki.GetCertPath(pki.KubeNodeCertName),
		ClientKey:        clientkey,
		ClientKeyPath:    pki.GetKeyPath(pki.KubeNodeCertName),
		ClientCA:         caCert,
		ClientCAPath:     pki.GetCertPath(pki.CACertName),
		KubeCfg:          clientConfig,
		ClusterCIDR:      c.ClusterCIDR,
		CNIImage:         c.Network.Options[CalicoCNIImage],
		NodeImage:        c.Network.Options[CalicoNodeImage],
		ControllersImage: c.Network.Options[CalicoControllersImage],
		Calicoctl:        c.Network.Options[CalicoctlImage],
		CloudProvider:    c.Network.Options[CalicoCloudProvider],
		RBACConfig:       c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(calicoConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName)
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
		NodeImage:       c.Network.Options[CanalNodeImage],
		CNIImage:        c.Network.Options[CanalCNIImage],
		CanalFlannelImg: c.Network.Options[CanalFlannelImage],
		RBACConfig:      c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(canalConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doWeaveDeploy(ctx context.Context) error {
	weaveConfig := map[string]string{
		ClusterCIDR: c.ClusterCIDR,
		Image:       c.Network.Options[WeaveImage],
		CNIImage:    c.Network.Options[WeaveCNIImage],
		RBACConfig:  c.Authorization.Mode,
	}
	pluginYaml, err := c.getNetworkPluginManifest(weaveConfig)
	if err != nil {
		return err
	}
	return c.doAddonDeploy(ctx, pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) setClusterNetworkDefaults() {
	setDefaultIfEmpty(&c.Network.Plugin, DefaultNetworkPlugin)

	if c.Network.Options == nil {
		// don't break if the user didn't define options
		c.Network.Options = make(map[string]string)
	}
	networkPluginConfigDefaultsMap := make(map[string]string)
	switch c.Network.Plugin {
	case FlannelNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			FlannelImage:    DefaultFlannelImage,
			FlannelCNIImage: DefaultFlannelCNIImage,
		}

	case CalicoNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			CalicoCNIImage:         DefaultCalicoCNIImage,
			CalicoNodeImage:        DefaultCalicoNodeImage,
			CalicoControllersImage: DefaultCalicoControllersImage,
			CalicoCloudProvider:    DefaultNetworkCloudProvider,
			CalicoctlImage:         DefaultCalicoctlImage,
		}

	case CanalNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			CanalCNIImage:     DefaultCanalCNIImage,
			CanalNodeImage:    DefaultCanalNodeImage,
			CanalFlannelImage: DefaultCanalFlannelImage,
		}

	case WeaveNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			WeaveImage:    DefaultWeaveImage,
			WeaveCNIImage: DefaultWeaveCNIImage,
		}
	}
	for k, v := range networkPluginConfigDefaultsMap {
		setDefaultIfEmptyMapValue(c.Network.Options, k, v)
	}

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
	if err := c.checkKubeAPIPort(ctx); err != nil {
		return err
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
	etcdPortList := []string{
		EtcdPort1,
		EtcdPort2,
	}
	if err := c.deployListenerOnPlane(ctx, etcdPortList, etcdHosts, EtcdPortListenContainer); err != nil {
		return err
	}

	// deploy controlplane listeners
	controlPlanePortList := []string{
		KubeAPIPort,
	}
	if err := c.deployListenerOnPlane(ctx, controlPlanePortList, cpHosts, CPPortListenContainer); err != nil {
		return err
	}

	// deploy worker listeners
	workerPortList := []string{
		KubeletPort,
	}
	if err := c.deployListenerOnPlane(ctx, workerPortList, workerHosts, WorkerPortListenContainer); err != nil {
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
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, containerName, host.Address, "network"); err != nil {
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
	etcdPortList := []string{
		EtcdPort1,
		EtcdPort2,
	}
	// one etcd host is a pass
	if len(c.EtcdHosts) > 1 {
		log.Infof(ctx, "[network] Running etcd <-> etcd port checks")
		for _, host := range c.EtcdHosts {
			runHost := host
			errgrp.Go(func() error {
				return checkPlaneTCPPortsFromHost(ctx, runHost, etcdPortList, c.EtcdHosts, c.SystemImages.Alpine)
			})
		}
		if err := errgrp.Wait(); err != nil {
			return err
		}
	}
	// check all -> etcd connectivity
	log.Infof(ctx, "[network] Running all -> etcd port checks")
	for _, host := range c.ControlPlaneHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, etcdPortList, c.EtcdHosts, c.SystemImages.Alpine)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	// Workers need to talk to etcd for calico
	for _, host := range c.WorkerHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, etcdPortList, c.EtcdHosts, c.SystemImages.Alpine)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	// check controle plane -> Workers
	log.Infof(ctx, "[network] Running control plane -> etcd port checks")
	workerPortList := []string{
		KubeletPort,
	}
	for _, host := range c.ControlPlaneHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, workerPortList, c.WorkerHosts, c.SystemImages.Alpine)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	// check workers -> control plane
	log.Infof(ctx, "[network] Running workers -> control plane port checks")
	controlPlanePortList := []string{
		KubeAPIPort,
	}
	for _, host := range c.WorkerHosts {
		runHost := host
		errgrp.Go(func() error {
			return checkPlaneTCPPortsFromHost(ctx, runHost, controlPlanePortList, c.ControlPlaneHosts, c.SystemImages.Alpine)
		})
	}
	return errgrp.Wait()
}

func checkPlaneTCPPortsFromHost(ctx context.Context, host *hosts.Host, portList []string, planeHosts []*hosts.Host, image string) error {
	hosts := []string{}
	for _, host := range planeHosts {
		hosts = append(hosts, host.InternalAddress)
	}
	imageCfg := &container.Config{
		Image: image,
		Tty:   true,
		Env: []string{
			fmt.Sprintf("HOSTS=%s", strings.Join(hosts, " ")),
			fmt.Sprintf("PORTS=%s", strings.Join(portList, " ")),
		},
		Cmd: []string{
			"sh",
			"-c",
			"for host in $HOSTS; do for port in $PORTS ; do nc -z $host $port > /dev/null || echo $host $port ; done; done",
		},
	}
	hostCfg := &container.HostConfig{
		NetworkMode: "host",
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, PortCheckContainer, host.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, PortCheckContainer, host.Address, "network"); err != nil {
		return err
	}
	if err := docker.WaitForContainer(ctx, host.DClient, PortCheckContainer); err != nil {
		return err
	}
	logs, err := docker.ReadContainerLogs(ctx, host.DClient, PortCheckContainer)
	if err != nil {
		return err
	}
	defer logs.Close()
	if err := docker.RemoveContainer(ctx, host.DClient, host.Address, PortCheckContainer); err != nil {
		return err
	}
	portCheckLogs, err := getPortCheckLogs(logs)
	if err != nil {
		return err
	}
	if len(portCheckLogs) > 0 {

		return fmt.Errorf("[network] Port check for ports: [%s] failed on host: [%s]", strings.Join(portCheckLogs, ", "), host.Address)

	}
	return nil
}

func getPortCheckLogs(reader io.ReadCloser) ([]string, error) {
	logLines := bufio.NewScanner(reader)
	hostPortLines := []string{}
	for logLines.Scan() {
		logLine := strings.Split(logLines.Text(), " ")
		hostPortLines = append(hostPortLines, fmt.Sprintf("%s:%s", logLine[0], logLine[1]))
	}
	if err := logLines.Err(); err != nil {
		return nil, err
	}
	return hostPortLines, nil
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
