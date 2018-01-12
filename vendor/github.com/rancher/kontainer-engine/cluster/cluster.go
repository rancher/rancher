package cluster

import (
	"context"
	"fmt"

	"github.com/rancher/kontainer-engine/logstream"
	"github.com/rancher/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	PreCreating = "Pre-Creating"
	Creating    = "Creating"
	PostCheck   = "Post-Checking"
	Running     = "Running"
	Error       = "Error"
	Updating    = "Updating"
)

// Cluster represents a kubernetes cluster
type Cluster struct {
	// The cluster driver to provision cluster
	Driver types.Driver `json:"-"`
	// The name of the cluster driver
	DriverName string `json:"driverName,omitempty" yaml:"driver_name,omitempty"`
	// The name of the cluster
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// The status of the cluster
	Status string `json:"status,omitempty" yaml:"status,omitempty"`

	// specific info about kubernetes cluster
	// Kubernetes cluster version
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Service account token to access kubernetes API
	ServiceAccountToken string `json:"serviceAccountToken,omitempty" yaml:"service_account_token,omitempty"`
	// Kubernetes API master endpoint
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	// Username for http basic authentication
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	// Password for http basic authentication
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	// Root CaCertificate for API server(base64 encoded)
	RootCACert string `json:"rootCACert,omitempty" yaml:"root_ca_cert,omitempty"`
	// Client Certificate(base64 encoded)
	ClientCertificate string `json:"clientCertificate,omitempty" yaml:"client_certificate,omitempty"`
	// Client private key(base64 encoded)
	ClientKey string `json:"clientKey,omitempty" yaml:"client_key,omitempty"`
	// Node count in the cluster
	NodeCount int64 `json:"nodeCount,omitempty" yaml:"node_count,omitempty"`

	// Metadata store specific driver options per cloud provider
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	PersistStore PersistStore `json:"-" yaml:"-"`

	ConfigGetter ConfigGetter `json:"-" yaml:"-"`

	Logger logstream.Logger `json:"-" yaml:"-"`
}

// PersistStore defines the interface for persist options like check and store
type PersistStore interface {
	GetStatus(name string) (string, error)
	Get(name string) (Cluster, error)
	Remove(name string) error
	Store(cluster Cluster) error
	PersistStatus(cluster Cluster, status string) error
}

// ConfigGetter defines the interface for getting the driver options.
type ConfigGetter interface {
	GetConfig() (types.DriverOptions, error)
}

// Create creates a cluster
func (c *Cluster) Create(ctx context.Context) error {
	if err := c.createInner(ctx); err != nil {
		if err := c.PersistStore.PersistStatus(*c, Error); err != nil {
			return err
		}
		return err
	}
	return c.PersistStore.PersistStatus(*c, Running)
}

func (c *Cluster) create(ctx context.Context) error {
	if c.Status == PostCheck {
		return nil
	}

	if err := c.PersistStore.PersistStatus(*c, PreCreating); err != nil {
		return err
	}

	// get cluster config from cli flags or json config
	driverOpts, err := c.ConfigGetter.GetConfig()
	if err != nil {
		return err
	}

	// also set metadata value to retrieve the cluster info
	for k, v := range c.Metadata {
		driverOpts.StringOptions[k] = v
	}

	if err := c.PersistStore.PersistStatus(*c, Creating); err != nil {
		return err
	}

	// create cluster
	info, err := c.Driver.Create(ctx, &driverOpts)
	if err != nil {
		return err
	}

	transformClusterInfo(c, info)
	return nil
}

func (c *Cluster) postCheck(ctx context.Context) error {
	if err := c.PersistStore.PersistStatus(*c, PostCheck); err != nil {
		return err
	}

	// receive cluster info back
	info, err := c.Driver.PostCheck(ctx, toInfo(c))
	if err != nil {
		return err
	}

	transformClusterInfo(c, info)

	// persist cluster info
	return c.Store()
}

func (c *Cluster) createInner(ctx context.Context) error {
	// check if it is already created
	c.restore()

	if c.Status == Error {
		logrus.Errorf("Cluster %s previously failed to create", c.Name)
		return fmt.Errorf("cluster %s previously failed to create", c.Name)
	}

	if c.Status == Updating || c.Status == Running {
		logrus.Infof("Cluster %s already exists.", c.Name)
		return nil
	}

	if err := c.create(ctx); err != nil {
		return err
	}

	return c.postCheck(ctx)
}

// Update updates a cluster
func (c *Cluster) Update(ctx context.Context) error {
	if err := c.restore(); err != nil {
		return err
	}

	if c.Status == Error {
		logrus.Errorf("Cluster %s previously failed to create", c.Name)
		return fmt.Errorf("cluster %s previously failed to create", c.Name)
	}

	if c.Status == PreCreating || c.Status == Creating {
		logrus.Errorf("Cluster %s host not been created.", c.Name)
		return fmt.Errorf("cluster %s host not been created", c.Name)
	}

	driverOpts, err := c.ConfigGetter.GetConfig()
	if err != nil {
		return err
	}
	driverOpts.StringOptions["name"] = c.Name
	for k, v := range c.Metadata {
		driverOpts.StringOptions[k] = v
	}

	if err := c.PersistStore.PersistStatus(*c, Updating); err != nil {
		return err
	}

	info := toInfo(c)
	info, err = c.Driver.Update(ctx, info, &driverOpts)
	if err != nil {
		return err
	}

	transformClusterInfo(c, info)

	return c.postCheck(ctx)
}

func transformClusterInfo(c *Cluster, clusterInfo *types.ClusterInfo) {
	c.ClientCertificate = clusterInfo.ClientCertificate
	c.ClientKey = clusterInfo.ClientKey
	c.RootCACert = clusterInfo.RootCaCertificate
	c.Username = clusterInfo.Username
	c.Password = clusterInfo.Password
	c.Version = clusterInfo.Version
	c.Endpoint = clusterInfo.Endpoint
	c.NodeCount = clusterInfo.NodeCount
	c.Metadata = clusterInfo.Metadata
	c.ServiceAccountToken = clusterInfo.ServiceAccountToken
}

func toInfo(c *Cluster) *types.ClusterInfo {
	return &types.ClusterInfo{
		ClientCertificate:   c.ClientCertificate,
		ClientKey:           c.ClientKey,
		RootCaCertificate:   c.RootCACert,
		Username:            c.Username,
		Password:            c.Password,
		Version:             c.Version,
		Endpoint:            c.Endpoint,
		NodeCount:           c.NodeCount,
		Metadata:            c.Metadata,
		ServiceAccountToken: c.ServiceAccountToken,
	}
}

// Remove removes a cluster
func (c *Cluster) Remove(ctx context.Context) error {
	defer c.PersistStore.Remove(c.Name)
	if err := c.restore(); errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return c.Driver.Remove(ctx, toInfo(c))
}

func (c *Cluster) getState() (string, error) {
	return c.PersistStore.GetStatus(c.Name)
}

// Store persists cluster information
func (c *Cluster) Store() error {
	return c.PersistStore.Store(*c)
}

func (c *Cluster) restore() error {
	cluster, err := c.PersistStore.Get(c.Name)
	if err != nil {
		return err
	}
	info := toInfo(&cluster)
	transformClusterInfo(c, info)
	return nil
}

// NewCluster create a cluster interface to do operations
func NewCluster(driverName, addr, name string, configGetter ConfigGetter, persistStore PersistStore) (*Cluster, error) {
	rpcClient, err := types.NewClient(driverName, addr)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		Driver:       rpcClient,
		DriverName:   driverName,
		Name:         name,
		ConfigGetter: configGetter,
		PersistStore: persistStore,
	}, nil
}

func FromCluster(cluster *Cluster, addr string, configGetter ConfigGetter, persistStore PersistStore) (*Cluster, error) {
	rpcClient, err := types.NewClient(cluster.DriverName, addr)
	if err != nil {
		return nil, err
	}
	cluster.Driver = rpcClient
	cluster.ConfigGetter = configGetter
	cluster.PersistStore = persistStore
	return cluster, nil
}
