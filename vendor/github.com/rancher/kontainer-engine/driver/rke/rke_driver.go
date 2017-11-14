package rke

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	generic "github.com/rancher/kontainer-engine/driver"
	"github.com/rancher/rke/cmd"
)

// Driver is the struct of rke driver
type Driver struct {
	// The string representation of Config Yaml
	ConfigYaml string
	// Kubernetes master endpoint
	Endpoint string
	// Root certificates
	RootCA string
	// Client certificates
	ClientCert string
	// Client key
	ClientKey string
	// Cluster info
	ClusterInfo generic.ClusterInfo
}

// NewDriver creates a new rke driver
func NewDriver() *Driver {
	return &Driver{}
}

// GetDriverCreateOptions returns create flags for rke driver
func (d *Driver) GetDriverCreateOptions() (*generic.DriverFlags, error) {
	driverFlag := generic.DriverFlags{
		Options: make(map[string]*generic.Flag),
	}
	driverFlag.Options["config-file-path"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "the path to the config file",
	}
	return &driverFlag, nil
}

// GetDriverUpdateOptions returns update flags for rke driver
func (d *Driver) GetDriverUpdateOptions() (*generic.DriverFlags, error) {
	driverFlag := generic.DriverFlags{
		Options: make(map[string]*generic.Flag),
	}
	driverFlag.Options["config-file-path"] = &generic.Flag{
		Type:  generic.StringType,
		Usage: "the path to the config file",
	}
	return &driverFlag, nil
}

// SetDriverOptions sets the drivers options to rke driver
func (d *Driver) SetDriverOptions(driverOptions *generic.DriverOptions) error {
	// first look up the file path then look up raw rkeConfig
	if path, ok := driverOptions.StringOptions["config-file-path"]; ok {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		d.ConfigYaml = string(data)
		return nil
	}
	d.ConfigYaml = driverOptions.StringOptions["rkeConfig"]
	return nil
}

// Create creates the rke cluster
func (d *Driver) Create() error {
	APIURL, caCrt, clientCert, clientKey, err := cmd.ClusterUp(d.ConfigYaml)
	if err != nil {
		return err
	}
	d.Endpoint = APIURL
	d.RootCA = caCrt
	d.ClientCert = clientCert
	d.ClientKey = clientKey
	return nil
}

// Update updates the rke cluster
func (d *Driver) Update() error {
	APIURL, caCrt, clientCert, clientKey, err := cmd.ClusterUp(d.ConfigYaml)
	if err != nil {
		return err
	}
	d.Endpoint = APIURL
	d.RootCA = caCrt
	d.ClientCert = clientCert
	d.ClientKey = clientKey
	return nil
}

// Get retrieve the cluster info by name
func (d *Driver) Get() (*generic.ClusterInfo, error) {
	return &d.ClusterInfo, nil
}

// PostCheck does post action
func (d *Driver) PostCheck() error {
	info := &generic.ClusterInfo{}
	info.Endpoint = d.Endpoint
	info.ClientCertificate = base64.StdEncoding.EncodeToString([]byte(d.ClientCert))
	info.ClientKey = base64.StdEncoding.EncodeToString([]byte(d.ClientKey))
	info.RootCaCertificate = base64.StdEncoding.EncodeToString([]byte(d.RootCA))

	host := d.Endpoint
	if !strings.HasPrefix(host, "https://") {
		host = fmt.Sprintf("https://%s", host)
	}
	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   []byte(d.RootCA),
			CertData: []byte(d.ClientCert),
			KeyData:  []byte(d.ClientKey),
		},
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	serverVersion, err := clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("Failed to get Kubernetes server version: %v", err)
	}
	token, err := generic.GenerateServiceAccountToken(clientset)
	if err != nil {
		return err
	}
	info.Version = serverVersion.GitVersion
	info.ServiceAccountToken = token
	d.ClusterInfo = *info
	return nil
}

// Remove removes the cluster
func (d *Driver) Remove() error {
	return cmd.ClusterRemove(d.ConfigYaml)
}
