package gke

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	generic "github.com/rancher/kontainer-engine/driver"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	raw "google.golang.org/api/container/v1"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	runningStatus        = "RUNNING"
	defaultCredentialEnv = "GOOGLE_APPLICATION_CREDENTIALS"
)

// Driver defines the struct of gke driver
type Driver struct {
	// ProjectID is the ID of your project to use when creating a cluster
	ProjectID string
	// The zone to launch the cluster
	Zone string
	// The IP address range of the container pods
	ClusterIpv4Cidr string
	// An optional description of this cluster
	Description string
	// The number of nodes to create in this cluster
	NodeCount int64
	// the kubernetes master version
	MasterVersion string
	// The authentication information for accessing the master
	MasterAuth *raw.MasterAuth
	// the kubernetes node version
	NodeVersion string
	// The name of this cluster
	Name string
	// Parameters used in creating the cluster's nodes
	NodeConfig *raw.NodeConfig
	// The path to the credential file(key.json)
	CredentialPath string
	// The content of the credential
	CredentialContent string
	// the temp file of the credential
	TempCredentialPath string
	// Enable alpha feature
	EnableAlphaFeature bool
	// Configuration for the HTTP (L7) load balancing controller addon
	HTTPLoadBalancing bool
	// Configuration for the horizontal pod autoscaling feature, which increases or decreases the number of replica pods a replication controller has based on the resource usage of the existing pods
	HorizontalPodAutoscaling bool
	// Configuration for the Kubernetes Dashboard
	KubernetesDashboard bool
	// Configuration for NetworkPolicy
	NetworkPolicyConfig bool
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
	// cluster info
	ClusterInfo generic.ClusterInfo
}

// NewDriver creates a gke Driver
func NewDriver() *Driver {
	return &Driver{
		NodeConfig: &raw.NodeConfig{
			Labels: map[string]string{},
		},
		ClusterInfo: generic.ClusterInfo{
			Metadata: map[string]string{},
		},
	}
}

// GetDriverCreateOptions implements driver interface
func (d *Driver) GetDriverCreateOptions() (*generic.DriverFlags, error) {
	driverFlag := generic.DriverFlags{
		Options: make(map[string]*generic.Flag),
	}
	driverFlag.Options["project-id"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "the ID of your project to use when creating a cluster",
	}
	driverFlag.Options["zone"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "The zone to launch the cluster",
		Value: "us-central1-a",
	}
	driverFlag.Options["gke-credential-path"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "the path to the credential json file(example: $HOME/key.json)",
	}
	driverFlag.Options["cluster-ipv4-cidr"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "The IP address range of the container pods",
	}
	driverFlag.Options["description"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "An optional description of this cluster",
	}
	driverFlag.Options["master-version"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "The kubernetes master version",
	}
	driverFlag.Options["node-count"] = &generic.Flag{
		Type:  generic.IntType,
		Usage: "The number of nodes to create in this cluster",
		Value: "3",
	}
	driverFlag.Options["disk-size-gb"] = &generic.Flag{
		Type:  generic.IntType,
		Usage: "Size of the disk attached to each node",
		Value: "100",
	}
	driverFlag.Options["labels"] = &generic.Flag{
		Type:  generic.StringSliceType,
		Usage: "The map of Kubernetes labels (key/value pairs) to be applied to each node",
	}
	driverFlag.Options["machine-type"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "The machine type of a Google Compute Engine",
	}
	driverFlag.Options["enable-alpha-feature"] = &generic.Flag{
		Type:  generic.BoolType,
		Usage: "To enable kubernetes alpha feature",
	}
	return &driverFlag, nil
}

// GetDriverUpdateOptions implements driver interface
func (d *Driver) GetDriverUpdateOptions() (*generic.DriverFlags, error) {
	driverFlag := generic.DriverFlags{
		Options: make(map[string]*generic.Flag),
	}
	driverFlag.Options["node-count"] = &generic.Flag{
		Type:  generic.IntType,
		Usage: "The node number for your cluster to update. 0 means no updates",
	}
	driverFlag.Options["master-version"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "The kubernetes master version to update",
	}
	driverFlag.Options["node-version"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "The kubernetes node version to update",
	}
	return &driverFlag, nil
}

// SetDriverOptions implements driver interface
func (d *Driver) SetDriverOptions(driverOptions *generic.DriverOptions) error {
	d.Name = getValueFromDriverOptions(driverOptions, generic.StringType, "name").(string)
	d.ProjectID = getValueFromDriverOptions(driverOptions, generic.StringType, "project-id", "projectId").(string)
	d.Zone = getValueFromDriverOptions(driverOptions, generic.StringType, "zone").(string)
	d.NodePoolID = getValueFromDriverOptions(driverOptions, generic.StringType, "nodePool").(string)
	d.ClusterIpv4Cidr = getValueFromDriverOptions(driverOptions, generic.StringType, "cluster-ipv4-cidr", "clusterIpv4Cidr").(string)
	d.Description = getValueFromDriverOptions(driverOptions, generic.StringType, "description").(string)
	d.MasterVersion = getValueFromDriverOptions(driverOptions, generic.StringType, "master-version", "masterVersion").(string)
	d.NodeVersion = getValueFromDriverOptions(driverOptions, generic.StringType, "node-version", "nodeVersion").(string)
	d.NodeConfig.DiskSizeGb = getValueFromDriverOptions(driverOptions, generic.IntType, "disk-size-gb", "diskSizeGb").(int64)
	d.NodeConfig.MachineType = getValueFromDriverOptions(driverOptions, generic.StringType, "machine-type", "machineType").(string)
	d.CredentialPath = getValueFromDriverOptions(driverOptions, generic.StringType, "gke-credential-path").(string)
	d.CredentialContent = getValueFromDriverOptions(driverOptions, generic.StringType, "credential").(string)
	d.EnableAlphaFeature = getValueFromDriverOptions(driverOptions, generic.BoolType, "enable-alpha-feature", "enableAlphaFeature").(bool)
	d.HorizontalPodAutoscaling = getValueFromDriverOptions(driverOptions, generic.BoolType, "horizontalPodAutoscaling").(bool)
	d.HTTPLoadBalancing = getValueFromDriverOptions(driverOptions, generic.BoolType, "httpLoadBalancing").(bool)
	d.KubernetesDashboard = getValueFromDriverOptions(driverOptions, generic.BoolType, "kubernetesDashboard").(bool)
	d.NetworkPolicyConfig = getValueFromDriverOptions(driverOptions, generic.BoolType, "networkPolicyConfig").(bool)
	d.NodeConfig.ImageType = getValueFromDriverOptions(driverOptions, generic.StringType, "imageType").(string)
	d.Network = getValueFromDriverOptions(driverOptions, generic.StringType, "network").(string)
	d.SubNetwork = getValueFromDriverOptions(driverOptions, generic.StringType, "subNetwork").(string)
	d.LegacyAbac = getValueFromDriverOptions(driverOptions, generic.BoolType, "legacyAbac").(bool)
	d.Locations = []string{}
	locations := getValueFromDriverOptions(driverOptions, generic.StringSliceType, "locations").(*generic.StringSlice)
	for _, location := range locations.Value {
		d.Locations = append(d.Locations, location)
	}

	d.NodeCount = getValueFromDriverOptions(driverOptions, generic.IntType, "node-count", "nodeCount").(int64)
	labelValues := getValueFromDriverOptions(driverOptions, generic.StringSliceType, "labels").(*generic.StringSlice)
	for _, part := range labelValues.Value {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			d.NodeConfig.Labels[kv[0]] = kv[1]
		}
	}
	if d.CredentialPath != "" {
		os.Setenv(defaultCredentialEnv, d.CredentialPath)
	}
	if d.CredentialContent != "" {
		file, err := ioutil.TempFile("", "credential-file")
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(file.Name(), []byte(d.CredentialContent), 0755); err != nil {
			return err
		}
		os.Setenv(defaultCredentialEnv, file.Name())
		d.TempCredentialPath = file.Name()
	}
	// updateConfig
	return d.validate()
}

func getValueFromDriverOptions(driverOptions *generic.DriverOptions, optionType string, keys ...string) interface{} {
	switch optionType {
	case generic.IntType:
		for _, key := range keys {
			if value, ok := driverOptions.IntOptions[key]; ok {
				return value
			}
		}
		return int64(0)
	case generic.StringType:
		for _, key := range keys {
			if value, ok := driverOptions.StringOptions[key]; ok {
				return value
			}
		}
		return ""
	case generic.BoolType:
		for _, key := range keys {
			if value, ok := driverOptions.BoolOptions[key]; ok {
				return value
			}
		}
		return false
	case generic.StringSliceType:
		for _, key := range keys {
			if value, ok := driverOptions.StringSliceOptions[key]; ok {
				return value
			}
		}
		return &generic.StringSlice{}
	}
	return nil
}

func (d *Driver) validate() error {
	if d.ProjectID == "" {
		return fmt.Errorf("project ID is required")
	} else if d.Zone == "" {
		return fmt.Errorf("zone is required")
	} else if d.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	return nil
}

// Create implements driver interface
func (d *Driver) Create() error {
	svc, err := d.getServiceClient()
	if err != nil {
		return err
	}
	operation, err := svc.Projects.Zones.Clusters.Create(d.ProjectID, d.Zone, d.generateClusterCreateRequest()).Context(context.Background()).Do()
	if err != nil && !strings.Contains(err.Error(), "alreadyExists") {
		return err
	}
	if err == nil {
		logrus.Debugf("Cluster %s create is called for project %s and zone %s. Status Code %v", d.Name, d.ProjectID, d.Zone, operation.HTTPStatusCode)
	}
	return d.waitCluster(svc)
}

// Update implements driver interface
func (d *Driver) Update() error {
	svc, err := d.getServiceClient()
	if err != nil {
		return err
	}
	logrus.Debugf("Updating config. MasterVersion: %s, NodeVersion: %s, NodeCount: %v", d.MasterVersion, d.NodeVersion, d.NodeCount)
	if d.NodePoolID == "" {
		cluster, err := svc.Projects.Zones.Clusters.Get(d.ProjectID, d.Zone, d.Name).Context(context.Background()).Do()
		if err != nil {
			return err
		}
		d.NodePoolID = cluster.NodePools[0].Name
	}

	if d.MasterVersion != "" {
		logrus.Infof("Updating master to %v", d.MasterVersion)
		operation, err := svc.Projects.Zones.Clusters.Update(d.ProjectID, d.Zone, d.Name, &raw.UpdateClusterRequest{
			Update: &raw.ClusterUpdate{
				DesiredMasterVersion: d.MasterVersion,
			},
		}).Context(context.Background()).Do()
		if err != nil {
			return err
		}
		logrus.Debugf("Cluster %s update is called for project %s and zone %s. Status Code %v", d.Name, d.ProjectID, d.Zone, operation.HTTPStatusCode)
		if err := d.waitCluster(svc); err != nil {
			return err
		}
	}

	if d.NodeVersion != "" {
		logrus.Infof("Updating node version to %v", d.NodeVersion)
		operation, err := svc.Projects.Zones.Clusters.NodePools.Update(d.ProjectID, d.Zone, d.Name, d.NodePoolID, &raw.UpdateNodePoolRequest{
			NodeVersion: d.NodeVersion,
		}).Context(context.Background()).Do()
		if err != nil {
			return err
		}
		logrus.Debugf("Nodepool %s update is called for project %s, zone %s and cluster %s. Status Code %v", d.NodePoolID, d.ProjectID, d.Zone, d.Name, operation.HTTPStatusCode)
		if err := d.waitNodePool(svc); err != nil {
			return err
		}
	}

	if d.NodeCount != 0 {
		logrus.Infof("Updating node number to %v", d.NodeCount)
		operation, err := svc.Projects.Zones.Clusters.NodePools.SetSize(d.ProjectID, d.Zone, d.Name, d.NodePoolID, &raw.SetNodePoolSizeRequest{
			NodeCount: d.NodeCount,
		}).Context(context.Background()).Do()
		if err != nil {
			return err
		}
		logrus.Debugf("Nodepool %s setSize is called for project %s, zone %s and cluster %s. Status Code %v", d.NodePoolID, d.ProjectID, d.Zone, d.Name, operation.HTTPStatusCode)
		if err := d.waitCluster(svc); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) generateClusterCreateRequest() *raw.CreateClusterRequest {
	request := raw.CreateClusterRequest{
		Cluster: &raw.Cluster{},
	}
	request.Cluster.Name = d.Name
	request.Cluster.Zone = d.Zone
	request.Cluster.InitialClusterVersion = d.MasterVersion
	request.Cluster.InitialNodeCount = d.NodeCount
	request.Cluster.ClusterIpv4Cidr = d.ClusterIpv4Cidr
	request.Cluster.Description = d.Description
	request.Cluster.EnableKubernetesAlpha = d.EnableAlphaFeature
	request.Cluster.AddonsConfig = &raw.AddonsConfig{
		HttpLoadBalancing:        &raw.HttpLoadBalancing{Disabled: !d.HTTPLoadBalancing},
		HorizontalPodAutoscaling: &raw.HorizontalPodAutoscaling{Disabled: !d.HorizontalPodAutoscaling},
		KubernetesDashboard:      &raw.KubernetesDashboard{Disabled: !d.KubernetesDashboard},
		NetworkPolicyConfig:      &raw.NetworkPolicyConfig{Disabled: !d.NetworkPolicyConfig},
	}
	request.Cluster.Network = d.Network
	request.Cluster.Subnetwork = d.SubNetwork
	request.Cluster.LegacyAbac = &raw.LegacyAbac{
		Enabled: d.LegacyAbac,
	}
	request.Cluster.MasterAuth = &raw.MasterAuth{
		Username: "admin",
	}
	request.Cluster.NodeConfig = d.NodeConfig
	return &request
}

// Get implements driver interface
func (d *Driver) Get() (*generic.ClusterInfo, error) {
	d.ClusterInfo.Metadata["project-id"] = d.ProjectID
	d.ClusterInfo.Metadata["zone"] = d.Zone
	d.ClusterInfo.Metadata["gke-credential-path"] = os.Getenv(defaultCredentialEnv)
	return &d.ClusterInfo, nil
}

func (d *Driver) PostCheck() error {
	svc, err := d.getServiceClient()
	if err != nil {
		return err
	}
	cluster, err := svc.Projects.Zones.Clusters.Get(d.ProjectID, d.Zone, d.Name).Context(context.Background()).Do()
	if err != nil {
		return err
	}
	d.ClusterInfo.Endpoint = cluster.Endpoint
	d.ClusterInfo.Version = cluster.CurrentMasterVersion
	d.ClusterInfo.Username = cluster.MasterAuth.Username
	d.ClusterInfo.Password = cluster.MasterAuth.Password
	d.ClusterInfo.RootCaCertificate = cluster.MasterAuth.ClusterCaCertificate
	d.ClusterInfo.ClientCertificate = cluster.MasterAuth.ClientCertificate
	d.ClusterInfo.ClientKey = cluster.MasterAuth.ClientKey
	d.ClusterInfo.NodeCount = cluster.CurrentNodeCount
	d.ClusterInfo.Metadata["nodePool"] = cluster.NodePools[0].Name
	serviceAccountToken, err := generateServiceAccountTokenForGke(cluster)
	if err != nil {
		return err
	}
	d.ClusterInfo.ServiceAccountToken = serviceAccountToken
	// clean up the default credential temp file
	os.RemoveAll(d.TempCredentialPath)
	return nil
}

// Remove implements driver interface
func (d *Driver) Remove() error {
	svc, err := d.getServiceClient()
	if err != nil {
		return err
	}
	logrus.Debugf("Removing cluster %v from project %v, zone %v", d.Name, d.ProjectID, d.Zone)
	operation, err := svc.Projects.Zones.Clusters.Delete(d.ProjectID, d.Zone, d.Name).Context(context.Background()).Do()
	if err != nil && !strings.Contains(err.Error(), "notFound") {
		return err
	} else if err == nil {
		logrus.Debugf("Cluster %v delete is called. Status Code %v", d.Name, operation.HTTPStatusCode)
	} else {
		logrus.Debugf("Cluster %s doesn't exist", d.Name)
	}
	os.RemoveAll(d.TempCredentialPath)
	return nil
}

func (d *Driver) getServiceClient() (*raw.Service, error) {
	client, err := google.DefaultClient(context.Background(), raw.CloudPlatformScope)
	if err != nil {
		return nil, err
	}
	service, err := raw.New(client)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func generateServiceAccountTokenForGke(cluster *raw.Cluster) (string, error) {
	capem, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return "", err
	}
	host := cluster.Endpoint
	if !strings.HasPrefix(host, "https://") {
		host = fmt.Sprintf("https://%s", host)
	}
	// in here we have to use http basic auth otherwise we can't get the permission to create cluster role
	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: capem,
		},
		Username: cluster.MasterAuth.Username,
		Password: cluster.MasterAuth.Password,
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	return generic.GenerateServiceAccountToken(clientset)
}

func (d *Driver) waitCluster(svc *raw.Service) error {
	lastMsg := ""
	for {
		cluster, err := svc.Projects.Zones.Clusters.Get(d.ProjectID, d.Zone, d.Name).Context(context.TODO()).Do()
		if err != nil {
			return err
		}
		if cluster.Status == runningStatus {
			logrus.Infof("Cluster %v is running", d.Name)
			return nil
		}
		if cluster.Status != lastMsg {
			logrus.Infof("%v cluster %v......", strings.ToLower(cluster.Status), d.Name)
			lastMsg = cluster.Status
		}
		time.Sleep(time.Second * 5)
	}
}

func (d *Driver) waitNodePool(svc *raw.Service) error {
	lastMsg := ""
	for {
		nodepool, err := svc.Projects.Zones.Clusters.NodePools.Get(d.ProjectID, d.Zone, d.Name, d.NodePoolID).Context(context.TODO()).Do()
		if err != nil {
			return err
		}
		if nodepool.Status == runningStatus {
			logrus.Infof("Nodepool %v is running", d.Name)
			return nil
		}
		if nodepool.Status != lastMsg {
			logrus.Infof("%v nodepool %v......", strings.ToLower(nodepool.Status), d.NodePoolID)
			lastMsg = nodepool.Status
		}
		time.Sleep(time.Second * 5)
	}
}
