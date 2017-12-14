package cluster

import (
	"fmt"

	"github.com/rancher/rke/network"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
)

const (
	NetworkPluginResourceName = "rke-network-plugin"

	FlannelNetworkPlugin = "flannel"
	FlannelImage         = "flannel_image"
	FlannelCNIImage      = "flannel_cni_image"
	FlannelIface         = "flannel_iface"

	CalicoNetworkPlugin     = "calico"
	CalicoNodeImage         = "calico_node_image"
	CalicoCNIImage          = "calico_cni_image"
	CalicoControllersImages = "calico_controllers_image"
	CalicoctlImage          = "calicoctl_image"
	CalicoCloudProvider     = "calico_cloud_provider"

	CanalNetworkPlugin = "canal"
	CanalNodeImage     = "canal_node_image"
	CanalCNIImage      = "canal_cni_image"
	CanalFlannelImage  = "canal_flannel_image"

	WeaveNetworkPlugin = "weave"
	WeaveImage         = "weave_node_image"
	WeaveCNIImage      = "weave_cni_image"
)

func (c *Cluster) DeployNetworkPlugin() error {
	logrus.Infof("[network] Setting up network plugin: %s", c.Network.Plugin)
	switch c.Network.Plugin {
	case FlannelNetworkPlugin:
		return c.doFlannelDeploy()
	case CalicoNetworkPlugin:
		return c.doCalicoDeploy()
	case CanalNetworkPlugin:
		return c.doCanalDeploy()
	case WeaveNetworkPlugin:
		return c.doWeaveDeploy()
	default:
		return fmt.Errorf("[network] Unsupported network plugin: %s", c.Network.Plugin)
	}
}

func (c *Cluster) doFlannelDeploy() error {
	flannelConfig := map[string]string{
		network.ClusterCIDR:     c.ClusterCIDR,
		network.FlannelImage:    c.Network.Options[FlannelImage],
		network.FlannelCNIImage: c.Network.Options[FlannelCNIImage],
		network.FlannelIface:    c.Network.Options[FlannelIface],
	}
	pluginYaml := network.GetFlannelManifest(flannelConfig)
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCalicoDeploy() error {
	calicoConfig := map[string]string{
		network.EtcdEndpoints:    services.GetEtcdConnString(c.EtcdHosts),
		network.APIRoot:          "https://127.0.0.1:6443",
		network.ClientCert:       pki.KubeNodeCertPath,
		network.ClientKey:        pki.KubeNodeKeyPath,
		network.ClientCA:         pki.CACertPath,
		network.KubeCfg:          pki.KubeNodeConfigPath,
		network.ClusterCIDR:      c.ClusterCIDR,
		network.CNIImage:         c.Network.Options[CalicoCNIImage],
		network.NodeImage:        c.Network.Options[CalicoNodeImage],
		network.ControllersImage: c.Network.Options[CalicoControllersImages],
		network.CalicoctlImage:   c.Network.Options[CalicoctlImage],
		network.CloudProvider:    c.Network.Options[CalicoCloudProvider],
	}
	pluginYaml := network.GetCalicoManifest(calicoConfig)
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCanalDeploy() error {
	canalConfig := map[string]string{
		network.ClientCert:   pki.KubeNodeCertPath,
		network.ClientKey:    pki.KubeNodeKeyPath,
		network.ClientCA:     pki.CACertPath,
		network.KubeCfg:      pki.KubeNodeConfigPath,
		network.ClusterCIDR:  c.ClusterCIDR,
		network.NodeImage:    c.Network.Options[CanalNodeImage],
		network.CNIImage:     c.Network.Options[CanalCNIImage],
		network.FlannelImage: c.Network.Options[CanalFlannelImage],
	}
	pluginYaml := network.GetCanalManifest(canalConfig)
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doWeaveDeploy() error {
	pluginYaml := network.GetWeaveManifest(c.ClusterCIDR, c.Network.Options[WeaveImage], c.Network.Options[WeaveCNIImage])
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) setClusterNetworkDefaults() {
	setDefaultIfEmpty(&c.Network.Plugin, DefaultNetworkPlugin)

	if c.Network.Options == nil {
		// don't break if the user didn't define options
		c.Network.Options = make(map[string]string)
	}
	networkPluginConfigDefaultsMap := make(map[string]string)
	switch {
	case c.Network.Plugin == FlannelNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			FlannelImage:    DefaultFlannelImage,
			FlannelCNIImage: DefaultFlannelCNIImage,
		}

	case c.Network.Plugin == CalicoNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			CalicoCNIImage:          DefaultCalicoCNIImage,
			CalicoNodeImage:         DefaultCalicoNodeImage,
			CalicoControllersImages: DefaultCalicoControllersImage,
			CalicoCloudProvider:     DefaultNetworkCloudProvider,
			CalicoctlImage:          DefaultCalicoctlImage,
		}

	case c.Network.Plugin == CanalNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			CanalCNIImage:     DefaultCanalCNIImage,
			CanalNodeImage:    DefaultCanalNodeImage,
			CanalFlannelImage: DefaultCanalFlannelImage,
		}

	case c.Network.Plugin == WeaveNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			WeaveImage:    DefaultWeaveImage,
			WeaveCNIImage: DefaultWeaveCNIImage,
		}
	}

	for k, v := range networkPluginConfigDefaultsMap {
		setDefaultIfEmptyMapValue(c.Network.Options, k, v)
	}

}
