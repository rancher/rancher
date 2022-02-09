package gke

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/options"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	raw "google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	runningStatus = "RUNNING"
	none          = "none"
)

// Driver defines the struct of gke driver
type Driver struct {
	driverCapabilities types.Capabilities
}

type state struct {
	// The displayed name of the cluster
	DisplayName string
	// ProjectID is the ID of your project to use when creating a cluster
	ProjectID string
	// The zone to launch the cluster
	Zone string
	// The region to launch the cluster
	Region string
	// The IP address range of the container pods
	ClusterIpv4Cidr string
	// An optional description of this cluster
	Description string
	// the kubernetes master version
	MasterVersion string
	// The authentication information for accessing the master
	MasterAuth *raw.MasterAuth
	// the kubernetes node version
	NodeVersion string
	// The name of this cluster
	Name string
	// Configuration options for the master authorized networks feature
	MasterAuthorizedNetworksConfig *raw.MasterAuthorizedNetworksConfig
	// The resource labels for the cluster to use to annotate any related Google Compute Engine resources
	ResourceLabels map[string]string
	// Configuration options for private clusters
	PrivateClusterConfig *raw.PrivateClusterConfig
	// NodePool contains the name and configuration for a cluster's node pool
	NodePool *raw.NodePool
	// Configuration for controlling how IPs are allocated in the cluster
	IPAllocationPolicy *raw.IPAllocationPolicy
	// The content of the credential
	CredentialContent string
	// Enable alpha feature
	EnableAlphaFeature bool
	// Configuration for the HTTP (L7) load balancing controller addon
	EnableHTTPLoadBalancing *bool
	// Configuration for the horizontal pod autoscaling feature, which increases or decreases the number of replica pods a replication controller has based on the resource usage of the existing pods
	EnableHorizontalPodAutoscaling *bool
	// Configuration for the Kubernetes Dashboard
	EnableKubernetesDashboard bool
	// Configuration for NetworkPolicy
	EnableNetworkPolicyConfig *bool
	// The list of Google Compute Engine locations in which the cluster's nodes should be located
	Locations []string
	// Network
	Network string
	// Sub Network
	SubNetwork string
	// Configuration for LegacyAbac
	LegacyAbac bool
	// NodePool id
	NodePoolID string

	EnableStackdriverLogging    *bool
	EnableStackdriverMonitoring *bool
	MaintenanceWindow           string

	// cluster info
	ClusterInfo types.ClusterInfo
}

// location returns either the zone or the region from the state
func (s state) location() string {
	if s.Region != "" {
		return s.Region
	}

	return s.Zone
}

func NewDriver() types.Driver {
	driver := &Driver{
		driverCapabilities: types.Capabilities{
			Capabilities: make(map[int64]bool),
		},
	}

	driver.driverCapabilities.AddCapability(types.GetVersionCapability)
	driver.driverCapabilities.AddCapability(types.SetVersionCapability)
	driver.driverCapabilities.AddCapability(types.GetClusterSizeCapability)
	driver.driverCapabilities.AddCapability(types.SetClusterSizeCapability)

	return driver
}

// GetDriverCreateOptions implements driver interface
func (d *Driver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the internal name of the cluster in Rancher",
	}
	driverFlag.Options["display-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the name of the cluster that should be displayed to the user",
	}
	driverFlag.Options["project-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the ID of your project to use when creating a cluster",
	}
	driverFlag.Options["zone"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The zone to launch the cluster",
	}
	driverFlag.Options["region"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The region to launch the cluster",
	}
	driverFlag.Options["cluster-ipv4-cidr"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP address range of the container pods",
	}
	driverFlag.Options["description"] = &types.Flag{
		Type:  types.StringType,
		Usage: "An optional description of this cluster",
	}
	driverFlag.Options["master-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The kubernetes master version",
	}
	driverFlag.Options["node-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The number of nodes to create in this cluster",
		Default: &types.Default{
			DefaultInt: 3,
		},
	}
	driverFlag.Options["disk-size-gb"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Size of the disk attached to each node",
		Default: &types.Default{
			DefaultInt: 100,
		},
	}
	driverFlag.Options["labels"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "The map of Kubernetes labels (key/value pairs) to be applied to each node",
	}
	driverFlag.Options["machine-type"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The machine type of a Google Compute Engine",
	}
	driverFlag.Options["enable-alpha-feature"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "To enable kubernetes alpha feature",
	}
	driverFlag.Options["enable-stackdriver-logging"] = &types.Flag{
		Type:  types.BoolPointerType,
		Usage: "Disable stackdriver logging",
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	driverFlag.Options["enable-stackdriver-monitoring"] = &types.Flag{
		Type:  types.BoolPointerType,
		Usage: "Disable stackdriver monitoring",
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	driverFlag.Options["kubernetes-dashboard"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Enable the kubernetes dashboard",
	}
	driverFlag.Options["maintenance-window"] = &types.Flag{
		Type:  types.StringType,
		Usage: "When to performance updates on the nodes, in 24-hour time (e.g. \"19:00\")",
	}
	driverFlag.Options["resource-labels"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "The map of Kubernetes labels (key/value pairs) to be applied to each cluster",
	}
	driverFlag.Options["enable-nodepool-autoscaling"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Enable nodepool autoscaling",
	}
	driverFlag.Options["min-node-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Minimmum number of nodes in the NodePool. Must be >= 1 and <= maxNodeCount",
	}
	driverFlag.Options["max-node-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Maximum number of nodes in the NodePool. Must be >= minNodeCount. There has to enough quota to scale up the cluster",
	}
	driverFlag.Options["enable-auto-upgrade"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Specifies whether node auto-upgrade is enabled for the node pool",
	}
	driverFlag.Options["enable-auto-repair"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Specifies whether the node auto-repair is enabled for the node pool",
	}
	driverFlag.Options["local-ssd-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "he number of local SSD disks to be attached to the node",
	}
	driverFlag.Options["preemptible"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether the nodes are created as preemptible VM instances",
	}
	driverFlag.Options["disk-type"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Type of the disk attached to each node",
	}
	driverFlag.Options["taints"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "List of kubernetes taints to be applied to each node",
	}
	driverFlag.Options["service-account"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The Google Cloud Platform Service Account to be used by the node VMs",
	}
	driverFlag.Options["oauth-scopes"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "The set of Google API scopes to be made available on all of the node VMs under the default service account",
	}
	driverFlag.Options["issue-client-certificate"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Issue a client certificate",
	}
	driverFlag.Options["enable-master-authorized-network"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether or not master authorized network is enabled",
	}
	driverFlag.Options["master-authorized-network-cidr-blocks"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Define up to 10 external networks that could access Kubernetes master through HTTPS",
	}
	driverFlag.Options["enable-private-endpoint"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether the master's internal IP address is used as the cluster endpoint",
	}
	driverFlag.Options["enable-private-nodes"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether nodes have internal IP address only",
	}
	driverFlag.Options["master-ipv4-cidr-block"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP range in CIDR notation to use for the hosted master network",
	}
	driverFlag.Options["use-ip-aliases"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether alias IPs will be used for pod IPs in the cluster",
	}
	driverFlag.Options["ip-policy-create-subnetwork"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether a new subnetwork will be created automatically for the cluster",
	}
	driverFlag.Options["ip-policy-subnetwork-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "A custom subnetwork name to be used if createSubnetwork is true",
	}
	driverFlag.Options["ip-policy-cluster-secondary-range-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of the secondary range to be used for the cluster CIDR block",
	}
	driverFlag.Options["ip-policy-services-secondary-range-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of the secondary range to be used for the services CIDR block",
	}
	driverFlag.Options["ip-policy-cluster-ipv4-cidr-block"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP address range for the cluster pod IPs",
	}
	driverFlag.Options["ip-policy-node-ipv4-cidr-block"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP address range of the instance IPs in this cluster",
	}
	driverFlag.Options["ip-policy-services-ipv4-cidr-block"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP address range of the services IPs in this cluster",
	}

	driverFlag.Options["node-pool"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The ID of the cluster node pool",
	}
	driverFlag.Options["node-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The version of kubernetes to use on the nodes",
	}
	driverFlag.Options["machine-type"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The machine type to use for the worker nodes",
	}
	driverFlag.Options["credential"] = &types.Flag{
		Type:     types.StringType,
		Password: true,
		Usage:    "The contents of the GC credential file",
	}
	driverFlag.Options["enable-kubernetes-dashboard"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether to enable the kubernetes dashboard",
	}
	driverFlag.Options["image-type"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The image to use for the worker nodes",
	}
	driverFlag.Options["network"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The network to use for the cluster",
	}
	driverFlag.Options["sub-network"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The sub-network to use for the cluster",
	}
	driverFlag.Options["enable-legacy-abac"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Whether to enable legacy abac on the cluster",
	}
	driverFlag.Options["locations"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Locations to use for the cluster",
	}
	driverFlag.Options["enable-horizontal-pod-autoscaling"] = &types.Flag{
		Type:  types.BoolPointerType,
		Usage: "Enable horizontal pod autoscaling for the cluster",
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	driverFlag.Options["enable-http-load-balancing"] = &types.Flag{
		Type:  types.BoolPointerType,
		Usage: "Enable http load balancing for the cluster",
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	driverFlag.Options["enable-network-policy-config"] = &types.Flag{
		Type:  types.BoolPointerType,
		Usage: "Enable network policy config for the cluster",
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	return &driverFlag, nil
}

// GetDriverUpdateOptions implements driver interface
func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["node-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The node number for your cluster to update. 0 means no updates",
	}
	driverFlag.Options["master-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The kubernetes master version to update",
	}
	driverFlag.Options["node-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The kubernetes node version to update",
	}
	driverFlag.Options["credential"] = &types.Flag{
		Type:     types.StringType,
		Password: true,
		Usage:    "The contents of the GC credential file",
	}
	return &driverFlag, nil
}

// SetDriverOptions implements driver interface
func getStateFromOpts(driverOptions *types.DriverOptions) (state, error) {
	d := state{
		ClusterInfo: types.ClusterInfo{
			Metadata: map[string]string{},
		},
		MasterAuth: &raw.MasterAuth{
			ClientCertificateConfig: &raw.ClientCertificateConfig{},
		},
		MasterAuthorizedNetworksConfig: &raw.MasterAuthorizedNetworksConfig{
			CidrBlocks: []*raw.CidrBlock{},
		},
		NodePool: &raw.NodePool{
			Autoscaling: &raw.NodePoolAutoscaling{},
			Config: &raw.NodeConfig{
				Labels:      map[string]string{},
				OauthScopes: []string{},
				Taints:      []*raw.NodeTaint{},
			},
			Management: &raw.NodeManagement{},
		},
		PrivateClusterConfig: &raw.PrivateClusterConfig{},
		IPAllocationPolicy:   &raw.IPAllocationPolicy{},
		ResourceLabels:       map[string]string{},
	}

	d.Name = options.GetValueFromDriverOptions(driverOptions, types.StringType, "name").(string)
	d.DisplayName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "display-name", "displayName").(string)
	d.ProjectID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "project-id", "projectId").(string)
	d.Zone = options.GetValueFromDriverOptions(driverOptions, types.StringType, "zone").(string)
	d.Region = options.GetValueFromDriverOptions(driverOptions, types.StringType, "region").(string)
	d.NodePoolID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "nodePool").(string)
	d.ClusterIpv4Cidr = options.GetValueFromDriverOptions(driverOptions, types.StringType, "cluster-ipv4-cidr", "clusterIpv4Cidr").(string)
	d.Description = options.GetValueFromDriverOptions(driverOptions, types.StringType, "description").(string)
	d.MasterVersion = options.GetValueFromDriverOptions(driverOptions, types.StringType, "master-version", "masterVersion").(string)
	d.NodeVersion = options.GetValueFromDriverOptions(driverOptions, types.StringType, "node-version", "nodeVersion").(string)
	d.NodePool.Config.DiskSizeGb = options.GetValueFromDriverOptions(driverOptions, types.IntType, "disk-size-gb", "diskSizeGb").(int64)
	d.NodePool.Config.MachineType = options.GetValueFromDriverOptions(driverOptions, types.StringType, "machine-type", "machineType").(string)
	d.NodePool.Config.DiskType = options.GetValueFromDriverOptions(driverOptions, types.StringType, "disk-type", "diskType").(string)
	d.NodePool.Config.LocalSsdCount = options.GetValueFromDriverOptions(driverOptions, types.IntType, "local-ssd-count", "localSsdCount").(int64)
	d.NodePool.Config.Preemptible = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "preemptible").(bool)
	d.NodePool.Config.ServiceAccount = options.GetValueFromDriverOptions(driverOptions, types.StringType, "service-account", "serviceAccount").(string)
	d.NodePool.InitialNodeCount = options.GetValueFromDriverOptions(driverOptions, types.IntType, "node-count", "nodeCount").(int64)
	d.NodePool.Management.AutoRepair = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-auto-repair", "enableAutoRepair").(bool)
	d.NodePool.Management.AutoUpgrade = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-auto-upgrade", "enableAutoUpgrade").(bool)
	d.CredentialContent = options.GetValueFromDriverOptions(driverOptions, types.StringType, "credential").(string)
	d.EnableAlphaFeature = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-alpha-feature", "enableAlphaFeature").(bool)
	d.EnableHorizontalPodAutoscaling, _ = options.GetValueFromDriverOptions(driverOptions, types.BoolPointerType, "enableHorizontalPodAutoscaling").(*bool)
	d.EnableNetworkPolicyConfig, _ = options.GetValueFromDriverOptions(driverOptions, types.BoolPointerType, "enableNetworkPolicyConfig").(*bool)
	d.EnableHTTPLoadBalancing, _ = options.GetValueFromDriverOptions(driverOptions, types.BoolPointerType, "enable-http-load-balancing", "enableHttpLoadBalancing").(*bool)
	d.EnableKubernetesDashboard = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "kubernetes-dashboard", "enableKubernetesDashboard").(bool)
	d.NodePool.Config.ImageType = options.GetValueFromDriverOptions(driverOptions, types.StringType, "imageType").(string)
	d.Network = options.GetValueFromDriverOptions(driverOptions, types.StringType, "network").(string)
	d.SubNetwork = options.GetValueFromDriverOptions(driverOptions, types.StringType, "subNetwork").(string)
	d.LegacyAbac = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-legacy-abac", "enableLegacyAbac").(bool)
	d.Locations = []string{}
	locations := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "locations").(*types.StringSlice)
	for _, location := range locations.Value {
		d.Locations = append(d.Locations, location)
	}

	labelValues := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "labels").(*types.StringSlice)
	for _, part := range labelValues.Value {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			d.NodePool.Config.Labels[kv[0]] = kv[1]
		}
	}
	resourceLabelValues := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "resource-labels", "resourceLabels").(*types.StringSlice)
	for _, part := range resourceLabelValues.Value {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			d.ResourceLabels[kv[0]] = kv[1]
		}
	}

	oauthScopesValues := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "oauth-scopes", "oauthScopes").(*types.StringSlice)
	for _, oauthScope := range oauthScopesValues.Value {
		d.NodePool.Config.OauthScopes = append(d.NodePool.Config.OauthScopes, oauthScope)
	}

	// Configuration of Taints
	taints := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "taints").(*types.StringSlice)
	for _, part := range taints.Value {
		taint := &raw.NodeTaint{}
		ekv := strings.Split(part, ":")
		if len(ekv) == 2 {
			taint.Effect = ekv[0]
			kv := strings.Split(ekv[1], "=")
			if len(kv) == 2 {
				taint.Key = kv[0]
				taint.Value = kv[1]
			}
		}

		d.NodePool.Config.Taints = append(d.NodePool.Config.Taints, taint)
	}

	// Configuration of NodePoolAutoscaling
	d.NodePool.Autoscaling.Enabled = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-nodepool-autoscaling", "enableNodepoolAutoscaling").(bool)
	d.NodePool.Autoscaling.MinNodeCount = options.GetValueFromDriverOptions(driverOptions, types.IntType, "min-node-count", "minNodeCount").(int64)
	d.NodePool.Autoscaling.MaxNodeCount = options.GetValueFromDriverOptions(driverOptions, types.IntType, "max-node-count", "maxNodeCount").(int64)
	d.NodePool.Name = "default-0"

	// Configuration of MasterAuth
	d.MasterAuth.Username = "admin"
	d.MasterAuth.ClientCertificateConfig.IssueClientCertificate = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "issue-client-certificate", "issueClientCertificate").(bool)

	// Configuration of MasterAuthorizedNetworksConfig
	cidrBlocks := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "master-authorized-network-cidr-blocks", "masterAuthorizedNetworkCidrBlocks").(*types.StringSlice)
	for _, cidrBlock := range cidrBlocks.Value {
		cb := &raw.CidrBlock{
			CidrBlock: cidrBlock,
		}
		d.MasterAuthorizedNetworksConfig.CidrBlocks = append(d.MasterAuthorizedNetworksConfig.CidrBlocks, cb)
	}
	d.MasterAuthorizedNetworksConfig.Enabled = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-master-authorized-network", "enableMasterAuthorizedNetwork").(bool)

	// Configuration of PrivateClusterConfig
	d.PrivateClusterConfig.EnablePrivateEndpoint = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-private-endpoint", "enablePrivateEndpoint").(bool)
	d.PrivateClusterConfig.EnablePrivateNodes = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-private-nodes", "enablePrivateNodes").(bool)
	d.PrivateClusterConfig.MasterIpv4CidrBlock = options.GetValueFromDriverOptions(driverOptions, types.StringType, "master-ipv4-cidr-block", "masterIpv4CidrBlock").(string)

	// Configuration of IPAllocationPolicy
	d.IPAllocationPolicy.UseIpAliases = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "use-ip-aliases", "useIpAliases").(bool)
	d.IPAllocationPolicy.CreateSubnetwork = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "ip-policy-create-subnetwork", "ipPolicyCreateSubnetwork").(bool)
	d.IPAllocationPolicy.SubnetworkName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ip-policy-subnetwork-name", "ipPolicySubnetworkName").(string)
	d.IPAllocationPolicy.ClusterSecondaryRangeName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ip-policy-cluster-secondary-range-name", "ipPolicyClusterSecondaryRangeName").(string)
	d.IPAllocationPolicy.ServicesSecondaryRangeName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ip-policy-services-secondary-range-name", "ipPolicyServicesSecondaryRangeName").(string)
	d.IPAllocationPolicy.ClusterIpv4CidrBlock = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ip-policy-cluster-ipv4-cidr-block", "ipPolicyClusterIpv4CidrBlock").(string)
	d.IPAllocationPolicy.NodeIpv4CidrBlock = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ip-policy-node-ipv4-cidr-block", "ipPolicyNodeIpv4CidrBlock").(string)
	d.IPAllocationPolicy.ServicesIpv4CidrBlock = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ip-policy-services-ipv4-cidr-block", "ipPolicyServicesIpv4CidrBlock").(string)

	d.EnableStackdriverLogging, _ = options.GetValueFromDriverOptions(driverOptions, types.BoolPointerType, "enable-stackdriver-logging", "enableStackdriverLogging").(*bool)
	d.EnableStackdriverMonitoring, _ = options.GetValueFromDriverOptions(driverOptions, types.BoolPointerType, "enable-stackdriver-monitoring", "enableStackdriverMonitoring").(*bool)
	d.MaintenanceWindow = options.GetValueFromDriverOptions(driverOptions, types.StringType, "maintenance-window", "maintenanceWindow").(string)

	return d, d.validate()
}

func (s *state) validate() error {
	if s.ProjectID == "" {
		return fmt.Errorf("project ID is required")
	} else if s.Zone == "" && s.Region == "" {
		return fmt.Errorf("zone or region is required")
	} else if s.Zone != "" && s.Region != "" {
		return fmt.Errorf("only one of zone or region must be specified")
	} else if s.Name == "" {
		return fmt.Errorf("cluster name is required")
	}

	if s.NodePool.Autoscaling.Enabled &&
		(s.NodePool.Autoscaling.MinNodeCount < 1 || s.NodePool.Autoscaling.MaxNodeCount < s.NodePool.Autoscaling.MinNodeCount) {
		return fmt.Errorf("minNodeCount in the NodePool must be >= 1 and <= maxNodeCount")
	}

	if s.PrivateClusterConfig != nil && s.PrivateClusterConfig.EnablePrivateEndpoint && !s.PrivateClusterConfig.EnablePrivateNodes {
		return fmt.Errorf("private endpoint requires private nodes")
	}

	return nil
}

// Create implements driver interface
func (d *Driver) Create(ctx context.Context, opts *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	state, err := getStateFromOpts(opts)
	if err != nil {
		return nil, err
	}

	info := &types.ClusterInfo{}
	err = storeState(info, state)
	if err != nil {
		return info, err
	}

	svc, err := getServiceClient(ctx, state.CredentialContent)
	if err != nil {
		return info, err
	}

	operation, err := svc.Projects.Locations.Clusters.Create(locationRRN(state.ProjectID, state.location()), d.generateClusterCreateRequest(state)).Context(ctx).Do()
	if err != nil && !strings.Contains(err.Error(), "alreadyExists") {
		return info, err
	}

	if err == nil {
		logrus.Debugf("Cluster %s create is called for project %s and region/zone %s. Status Code %v", state.Name, state.ProjectID, state.location(), operation.HTTPStatusCode)
	}
	if err := d.waitCluster(ctx, svc, &state); err != nil {
		return info, err
	}
	return info, nil
}

func storeState(info *types.ClusterInfo, state state) error {
	bytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if info.Metadata == nil {
		info.Metadata = map[string]string{}
	}
	info.Metadata["state"] = string(bytes)
	info.Metadata["project-id"] = state.ProjectID
	info.Metadata["zone"] = state.Zone
	info.Metadata["region"] = state.Region
	return nil
}

func getState(info *types.ClusterInfo) (state, error) {
	state := state{}
	// ignore error
	err := json.Unmarshal([]byte(info.Metadata["state"]), &state)
	return state, err
}

// Update implements driver interface
func (d *Driver) Update(ctx context.Context, info *types.ClusterInfo, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	state, err := getState(info)
	if err != nil {
		return nil, err
	}

	newState, err := getStateFromOpts(opts)
	if err != nil {
		return nil, err
	}

	svc, err := getServiceClient(ctx, state.CredentialContent)
	if err != nil {
		return nil, err
	}

	if state.NodePoolID == "" {
		cluster, err := svc.Projects.Locations.Clusters.Get(clusterRRN(state.ProjectID, state.location(), state.Name)).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		state.NodePoolID = cluster.NodePools[0].Name
	}

	if state.NodePool != nil {
		logrus.Debugf("Updating config. MasterVersion: %s, NodeVersion: %s, NodeCount: %v", state.MasterVersion, state.NodeVersion, state.NodePool.InitialNodeCount)
	} else {
		logrus.Debugf("Updating config. MasterVersion: %s, NodeVersion: %s", state.MasterVersion, state.NodeVersion)
	}

	if newState.MasterVersion != "" {
		log.Infof(ctx, "Updating master to %v", newState.MasterVersion)
		operation, err := svc.Projects.Locations.Clusters.Update(
			clusterRRN(state.ProjectID, state.location(), state.Name), &raw.UpdateClusterRequest{
				Update: &raw.ClusterUpdate{
					DesiredMasterVersion: newState.MasterVersion,
				},
			}).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Cluster %s update is called for project %s and region/zone %s. Status Code %v", state.Name, state.ProjectID, state.location(), operation.HTTPStatusCode)
		if err := d.waitCluster(ctx, svc, &state); err != nil {
			return nil, err
		}
		state.MasterVersion = newState.MasterVersion
	}

	if newState.NodeVersion != "" {
		log.Infof(ctx, "Updating node version to %v", newState.NodeVersion)
		operation, err := svc.Projects.Locations.Clusters.NodePools.Update(
			nodePoolRRN(state.ProjectID, state.location(), state.Name, state.NodePoolID), &raw.UpdateNodePoolRequest{
				NodeVersion: state.NodeVersion,
			}).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Nodepool %s update is called for project %s, region/zone %s and cluster %s. Status Code %v", state.NodePoolID, state.ProjectID, state.location(), state.Name, operation.HTTPStatusCode)
		if err := d.waitNodePool(ctx, svc, &state); err != nil {
			return nil, err
		}
		state.NodeVersion = newState.NodeVersion
	}

	if newState.NodePool != nil && newState.NodePool.InitialNodeCount != 0 {
		log.Infof(ctx, "Updating node number to %v", newState.NodePool.InitialNodeCount)
		operation, err := svc.Projects.Locations.Clusters.NodePools.SetSize(
			nodePoolRRN(state.ProjectID, state.location(), state.Name, state.NodePoolID), &raw.SetNodePoolSizeRequest{
				NodeCount: newState.NodePool.InitialNodeCount,
			}).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Nodepool %s setSize is called for project %s, region/zone %s and cluster %s. Status Code %v", state.NodePoolID, state.ProjectID, state.location(), state.Name, operation.HTTPStatusCode)
		if err := d.waitCluster(ctx, svc, &state); err != nil {
			return nil, err
		}
	}

	if newState.NodePool != nil && newState.NodePool.Autoscaling != nil && newState.NodePool.Autoscaling.Enabled {
		log.Infof(ctx, "Updating the autoscaling settings for node pool %s", state.NodePoolID)
		operation, err := svc.Projects.Locations.Clusters.NodePools.SetAutoscaling(
			nodePoolRRN(state.ProjectID, state.location(), state.Name, state.NodePoolID), &raw.SetNodePoolAutoscalingRequest{
				Autoscaling: newState.NodePool.Autoscaling,
			}).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Nodepool %s autoscaling is called for project %s, region/zone %s and cluster %s. Status Code %v", state.NodePoolID, state.ProjectID, state.location(), state.Name, operation.HTTPStatusCode)
		if err := d.waitCluster(ctx, svc, &state); err != nil {
			return nil, err
		}
	}

	return info, storeState(info, state)
}

func (d *Driver) generateClusterCreateRequest(state state) *raw.CreateClusterRequest {
	request := raw.CreateClusterRequest{
		Cluster: &raw.Cluster{
			NodePools: []*raw.NodePool{},
		},
	}
	request.Cluster.Name = state.Name
	request.Cluster.InitialClusterVersion = state.MasterVersion
	request.Cluster.Description = state.Description
	request.Cluster.EnableKubernetesAlpha = state.EnableAlphaFeature
	request.Cluster.ClusterIpv4Cidr = state.ClusterIpv4Cidr

	disableHTTPLoadBalancing := state.EnableHTTPLoadBalancing != nil && !*state.EnableHTTPLoadBalancing
	disableHorizontalPodAutoscaling := state.EnableHorizontalPodAutoscaling != nil && !*state.EnableHorizontalPodAutoscaling
	disableNetworkPolicyConfig := state.EnableNetworkPolicyConfig != nil && !*state.EnableNetworkPolicyConfig

	request.Cluster.AddonsConfig = &raw.AddonsConfig{
		HttpLoadBalancing:        &raw.HttpLoadBalancing{Disabled: disableHTTPLoadBalancing},
		HorizontalPodAutoscaling: &raw.HorizontalPodAutoscaling{Disabled: disableHorizontalPodAutoscaling},
		KubernetesDashboard:      &raw.KubernetesDashboard{Disabled: !state.EnableKubernetesDashboard},
		NetworkPolicyConfig:      &raw.NetworkPolicyConfig{Disabled: disableNetworkPolicyConfig},
	}
	request.Cluster.Network = state.Network
	request.Cluster.Subnetwork = state.SubNetwork
	request.Cluster.LegacyAbac = &raw.LegacyAbac{
		Enabled: state.LegacyAbac,
	}
	request.Cluster.MasterAuth = state.MasterAuth
	request.Cluster.NodePools = append(request.Cluster.NodePools, state.NodePool)

	state.ResourceLabels["display-name"] = strings.ToLower(state.DisplayName)
	request.Cluster.ResourceLabels = state.ResourceLabels

	if state.MasterAuthorizedNetworksConfig.Enabled {
		request.Cluster.MasterAuthorizedNetworksConfig = state.MasterAuthorizedNetworksConfig
	}

	if state.PrivateClusterConfig != nil && state.PrivateClusterConfig.EnablePrivateNodes {
		request.Cluster.PrivateClusterConfig = state.PrivateClusterConfig
	}

	request.Cluster.IpAllocationPolicy = state.IPAllocationPolicy
	if request.Cluster.IpAllocationPolicy.UseIpAliases == true &&
		request.Cluster.IpAllocationPolicy.ClusterIpv4CidrBlock != "" {
		request.Cluster.ClusterIpv4Cidr = ""
	}

	// Stackdriver logging and monitoring default to "on" if no parameter is
	// passed in.  We must explicitly pass "none" if it isn't wanted
	if state.EnableStackdriverLogging != nil && !*state.EnableStackdriverLogging {
		request.Cluster.LoggingService = none
	}
	if state.EnableStackdriverMonitoring != nil && !*state.EnableStackdriverMonitoring {
		request.Cluster.MonitoringService = none
	}
	if state.MaintenanceWindow != "" {
		request.Cluster.MaintenancePolicy = &raw.MaintenancePolicy{
			Window: &raw.MaintenanceWindow{
				DailyMaintenanceWindow: &raw.DailyMaintenanceWindow{
					StartTime: state.MaintenanceWindow,
				},
			},
		}
	}
	request.Cluster.Locations = state.Locations

	return &request
}

func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	state, err := getState(info)
	if err != nil {
		return nil, err
	}

	ts, err := GetTokenSource(ctx, state.CredentialContent)
	if err != nil {
		return nil, err
	}

	svc, err := getServiceClientWithTokenSource(ctx, ts)
	if err != nil {
		return nil, err
	}

	if err := d.waitCluster(ctx, svc, &state); err != nil {
		return nil, err
	}

	cluster, err := svc.Projects.Locations.Clusters.Get(clusterRRN(state.ProjectID, state.location(), state.Name)).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	token, err := ts.Token()
	if err != nil {
		return nil, err
	}

	info.Endpoint = cluster.Endpoint
	info.Version = cluster.CurrentMasterVersion
	info.Username = cluster.MasterAuth.Username
	info.Password = cluster.MasterAuth.Password
	info.RootCaCertificate = cluster.MasterAuth.ClusterCaCertificate
	info.ClientCertificate = cluster.MasterAuth.ClientCertificate
	info.ClientKey = cluster.MasterAuth.ClientKey
	info.NodeCount = cluster.CurrentNodeCount
	info.Metadata["nodePool"] = cluster.NodePools[0].Name
	info.ServiceAccountToken = token.AccessToken
	return info, nil
}

// Remove implements driver interface
func (d *Driver) Remove(ctx context.Context, info *types.ClusterInfo) error {
	state, err := getState(info)
	if err != nil {
		return err
	}

	svc, err := getServiceClient(ctx, state.CredentialContent)
	if err != nil {
		return err
	}

	logrus.Debugf("Removing cluster %v from project %v, region/zone %v", state.Name, state.ProjectID, state.location())
	operation, err := d.waitClusterRemoveExp(ctx, svc, &state)
	if err != nil && !strings.Contains(err.Error(), "notFound") {
		return err
	} else if err == nil {
		logrus.Debugf("Cluster %v delete is called. Status Code %v", state.Name, operation.HTTPStatusCode)
	} else {
		logrus.Debugf("Cluster %s doesn't exist", state.Name)
	}
	return nil
}

func GetTokenSource(ctx context.Context, credential string) (oauth2.TokenSource, error) {
	ts, err := google.CredentialsFromJSON(ctx, []byte(credential), raw.CloudPlatformScope)
	if err != nil {
		return nil, err
	}
	return ts.TokenSource, nil
}

func getServiceClientWithTokenSource(ctx context.Context, ts oauth2.TokenSource) (*raw.Service, error) {
	client := oauth2.NewClient(ctx, ts)
	return raw.NewService(ctx, option.WithHTTPClient(client))
}

func getServiceClient(ctx context.Context, credential string) (*raw.Service, error) {
	ts, err := GetTokenSource(ctx, credential)
	if err != nil {
		return nil, err
	}
	return getServiceClientWithTokenSource(ctx, ts)
}

func getClientset(cluster *raw.Cluster, ts oauth2.TokenSource) (kubernetes.Interface, error) {
	capem, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, err
	}
	host := cluster.Endpoint
	if !strings.HasPrefix(host, "https://") {
		host = fmt.Sprintf("https://%s", host)
	}

	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: capem,
		},
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			return &oauth2.Transport{
				Source: ts,
				Base:   rt,
			}
		},
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func (d *Driver) waitCluster(ctx context.Context, svc *raw.Service, state *state) error {
	lastMsg := ""
	for {
		cluster, err := svc.Projects.Locations.Clusters.Get(clusterRRN(state.ProjectID, state.location(), state.Name)).Context(ctx).Do()
		if err != nil {
			return err
		}
		if cluster.Status == runningStatus {
			log.Infof(ctx, "Cluster %v is running", state.Name)
			return nil
		}
		if cluster.Status != lastMsg {
			log.Infof(ctx, "%v cluster %v......", strings.ToLower(cluster.Status), state.Name)
			lastMsg = cluster.Status
		}
		time.Sleep(time.Second * 5)
	}
}

func (d *Driver) waitClusterRemoveExp(ctx context.Context, svc *raw.Service, state *state) (*raw.Operation, error) {
	var operation *raw.Operation
	var err error

	for i := 1; i < 12; i++ {
		time.Sleep(time.Duration(i*i) * time.Second)
		operation, err = svc.Projects.Locations.Clusters.Delete(clusterRRN(state.ProjectID, state.location(), state.Name)).Context(ctx).Do()
		if err == nil {
			return operation, nil
		} else if !strings.Contains(err.Error(), "Please wait and try again once it is done") {
			break
		}
	}
	return operation, err
}

func (d *Driver) waitNodePool(ctx context.Context, svc *raw.Service, state *state) error {
	lastMsg := ""
	for {
		nodepool, err := svc.Projects.Locations.Clusters.NodePools.Get(
			nodePoolRRN(state.ProjectID, state.location(), state.Name, state.NodePoolID)).Context(ctx).Do()
		if err != nil {
			return err
		}
		if nodepool.Status == runningStatus {
			log.Infof(ctx, "Nodepool %v is running", state.Name)
			return nil
		}
		if nodepool.Status != lastMsg {
			log.Infof(ctx, "%v nodepool %v......", strings.ToLower(nodepool.Status), state.NodePoolID)
			lastMsg = nodepool.Status
		}
		time.Sleep(time.Second * 5)
	}
}

func (d *Driver) getClusterStats(ctx context.Context, info *types.ClusterInfo) (*raw.Cluster, error) {
	state, err := getState(info)
	if err != nil {
		return nil, err
	}

	svc, err := getServiceClient(ctx, state.CredentialContent)
	if err != nil {
		return nil, err
	}

	cluster, err := svc.Projects.Zones.Clusters.Get(state.ProjectID, state.location(), state.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster info: %v", err)
	}

	return cluster, nil
}

func (d *Driver) GetClusterSize(ctx context.Context, info *types.ClusterInfo) (*types.NodeCount, error) {
	cluster, err := d.getClusterStats(ctx, info)
	if err != nil {
		return nil, err
	}

	version := &types.NodeCount{Count: int64(cluster.NodePools[0].InitialNodeCount)}

	return version, nil
}

func (d *Driver) GetVersion(ctx context.Context, info *types.ClusterInfo) (*types.KubernetesVersion, error) {
	cluster, err := d.getClusterStats(ctx, info)
	if err != nil {
		return nil, err
	}

	version := &types.KubernetesVersion{Version: cluster.CurrentMasterVersion}

	return version, nil
}

func (d *Driver) SetClusterSize(ctx context.Context, info *types.ClusterInfo, count *types.NodeCount) error {
	cluster, err := d.getClusterStats(ctx, info)
	if err != nil {
		return err
	}

	state, err := getState(info)
	if err != nil {
		return err
	}

	client, err := getServiceClient(ctx, state.CredentialContent)
	if err != nil {
		return err
	}

	logrus.Infof("[googlekubernetesengine] updating cluster [%s] size", cluster.Name)

	_, err = client.Projects.Locations.Clusters.NodePools.SetSize(
		nodePoolRRN(state.ProjectID, state.location(), cluster.Name, cluster.NodePools[0].Name), &raw.SetNodePoolSizeRequest{
			NodeCount: count.Count,
		}).Context(ctx).Do()
	if err != nil {
		return err
	}

	err = d.waitCluster(ctx, client, &state)
	if err != nil {
		return err
	}

	logrus.Infof("[googlekubernetesengine] cluster [%s] size updated successfully", cluster.Name)

	return nil
}

func (d *Driver) SetVersion(ctx context.Context, info *types.ClusterInfo, version *types.KubernetesVersion) error {
	logrus.Info("[googlekubernetesengine] updating master version")

	err := d.updateAndWait(ctx, info, &raw.UpdateClusterRequest{
		Update: &raw.ClusterUpdate{
			DesiredMasterVersion: version.Version,
		}})
	if err != nil {
		return err
	}

	logrus.Info("[googlekubernetesengine] master version updated successfully")
	logrus.Info("[googlekubernetesengine] updating node version")

	err = d.updateAndWait(ctx, info, &raw.UpdateClusterRequest{
		Update: &raw.ClusterUpdate{
			DesiredNodeVersion: version.Version,
		},
	})
	if err != nil {
		return err
	}

	logrus.Info("[googlekubernetesengine] node version updated successfully")

	return nil
}

func (d *Driver) updateAndWait(ctx context.Context, info *types.ClusterInfo, updateRequest *raw.UpdateClusterRequest) error {
	cluster, err := d.getClusterStats(ctx, info)
	if err != nil {
		return err
	}

	state, err := getState(info)
	if err != nil {
		return err
	}

	client, err := getServiceClient(ctx, state.CredentialContent)
	if err != nil {
		return err
	}

	_, err = client.Projects.Locations.Clusters.Update(clusterRRN(state.ProjectID, state.location(), cluster.Name), updateRequest).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error while updating cluster: %v", err)
	}

	return d.waitCluster(ctx, client, &state)
}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
}

func (d *Driver) ETCDSave(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) ETCDRestore(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) (*types.ClusterInfo, error) {
	return nil, fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) ETCDRemoveSnapshot(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) GetK8SCapabilities(ctx context.Context, options *types.DriverOptions) (*types.K8SCapabilities, error) {
	state, err := getStateFromOpts(options)
	if err != nil {
		return nil, err
	}

	capabilities := &types.K8SCapabilities{
		L4LoadBalancer: &types.LoadBalancerCapabilities{
			Enabled:              true,
			Provider:             "GCLB",
			ProtocolsSupported:   []string{"TCP", "UDP"},
			HealthCheckSupported: true,
		},
	}
	if state.EnableHTTPLoadBalancing != nil && *state.EnableHTTPLoadBalancing {
		capabilities.IngressControllers = []*types.IngressCapabilities{
			{
				IngressProvider:      "GCLB",
				CustomDefaultBackend: true,
			},
		}
	}
	return capabilities, nil
}

func (d *Driver) RemoveLegacyServiceAccount(ctx context.Context, info *types.ClusterInfo) error {
	state, err := getState(info)
	if err != nil {
		return err
	}
	ts, err := GetTokenSource(ctx, state.CredentialContent)
	if err != nil {
		return err
	}

	svc, err := getServiceClientWithTokenSource(ctx, ts)
	if err != nil {
		return err
	}

	cluster, err := svc.Projects.Locations.Clusters.Get(clusterRRN(state.ProjectID, state.location(), state.Name)).Context(ctx).Do()
	if err != nil {
		return err
	}

	clientset, err := getClientset(cluster, ts)
	if err != nil {
		return err
	}

	err = util.DeleteLegacyServiceAccountAndRoleBinding(clientset)
	if err != nil {
		return err
	}

	return nil
}

func Oauth2Transport(ctx context.Context, rt http.RoundTripper, credentials string) (http.RoundTripper, error) {
	ts, err := GetTokenSource(ctx, credentials)
	if err != nil {
		return rt, fmt.Errorf("unable to retrieve token source for GKE oauth2: %v", err)
	}

	return &oauth2.Transport{Source: ts, Base: rt}, nil
}

// locationRRN returns a Relative Resource Name representing a location. This
// RRN can either represent a Region or a Zone. It can be used as the parent
// attribute during cluster creation to create a zonal or regional cluster, or
// be used to generate more specific RRNs like an RRN representing a cluster.
//
// https://cloud.google.com/apis/design/resource_names#relative_resource_name
func locationRRN(projectID, location string) string {
	return fmt.Sprintf("projects/%s/locations/%s", projectID, location)
}

// clusterRRN returns an Relative Resource Name of a cluster in the specified
// region or zone
func clusterRRN(projectID, location, clusterName string) string {
	return fmt.Sprintf("%s/clusters/%s", locationRRN(projectID, location), clusterName)
}

// nodePoolRRN returns a Relative Resource Name of a node pool in a cluster in the
// region or zone for the specified project
func nodePoolRRN(projectID, location, clusterName, nodePool string) string {
	return fmt.Sprintf("%s/nodePools/%s", clusterRRN(projectID, location, clusterName), nodePool)
}
