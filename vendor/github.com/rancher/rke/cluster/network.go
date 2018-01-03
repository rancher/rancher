package cluster

import (
	"fmt"

	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/templates"
	"github.com/sirupsen/logrus"
)

const (
	NetworkPluginResourceName = "rke-network-plugin"

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

	ClientCert = "ClientCert"
	ClientKey  = "ClientKey"
	ClientCA   = "ClientCA"
	KubeCfg    = "KubeCfg"

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
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCalicoDeploy() error {
	calicoConfig := map[string]string{
		EtcdEndpoints:    services.GetEtcdConnString(c.EtcdHosts),
		APIRoot:          "https://127.0.0.1:6443",
		ClientCert:       pki.KubeNodeCertPath,
		ClientKey:        pki.KubeNodeKeyPath,
		ClientCA:         pki.CACertPath,
		KubeCfg:          pki.KubeNodeConfigPath,
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
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCanalDeploy() error {
	canalConfig := map[string]string{
		ClientCert:      pki.KubeNodeCertPath,
		APIRoot:         "https://127.0.0.1:6443",
		ClientKey:       pki.KubeNodeKeyPath,
		ClientCA:        pki.CACertPath,
		KubeCfg:         pki.KubeNodeConfigPath,
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
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doWeaveDeploy() error {
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
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
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
