package rke

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/kontainer-engine/drivers"
	"github.com/rancher/kontainer-engine/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rke/cmd"
	"github.com/rancher/rke/hosts"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	kubeConfigFile = ".kube_config_cluster.yml"
	rancherPath    = "/var/lib/rancher/rke/"
)

// Driver is the struct of rke driver
type Driver struct {
	DockerDialer hosts.DialerFactory
}

// NewDriver creates a new rke driver
func NewDriver() *Driver {
	return &Driver{}
}

// GetDriverCreateOptions returns create flags for rke driver
func (d *Driver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["config-file-path"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the path to the config file",
	}
	return &driverFlag, nil
}

// GetDriverUpdateOptions returns update flags for rke driver
func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["config-file-path"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the path to the config file",
	}
	return &driverFlag, nil
}

// SetDriverOptions sets the drivers options to rke driver
func getYAML(driverOptions *types.DriverOptions) (string, error) {
	// first look up the file path then look up raw rkeConfig
	if path, ok := driverOptions.StringOptions["config-file-path"]; ok {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return driverOptions.StringOptions["rkeConfig"], nil
}

// Create creates the rke cluster
func (d *Driver) Create(ctx context.Context, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	yaml, err := getYAML(opts)
	if err != nil {
		return nil, err
	}

	rkeConfig, err := drivers.ConvertToRkeConfig(yaml)
	if err != nil {
		return nil, err
	}

	stateDir, err := d.restore(nil)
	if err != nil {
		return nil, err
	}
	APIURL, caCrt, clientCert, clientKey, err := cmd.ClusterUp(ctx, &rkeConfig, d.DockerDialer, nil, false, stateDir)
	if err != nil {
		return d.save(&types.ClusterInfo{
			Metadata: map[string]string{
				"Config": yaml,
			},
		}, stateDir), err
	}

	return d.save(&types.ClusterInfo{
		Metadata: map[string]string{
			"Endpoint":   APIURL,
			"RootCA":     base64.StdEncoding.EncodeToString([]byte(caCrt)),
			"ClientCert": base64.StdEncoding.EncodeToString([]byte(clientCert)),
			"ClientKey":  base64.StdEncoding.EncodeToString([]byte(clientKey)),
			"Config":     yaml,
		},
	}, stateDir), nil
}

// Update updates the rke cluster
func (d *Driver) Update(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	yaml, err := getYAML(opts)
	if err != nil {
		return nil, err
	}

	rkeConfig, err := drivers.ConvertToRkeConfig(yaml)
	if err != nil {
		return nil, err
	}

	stateDir, err := d.restore(clusterInfo)
	if err != nil {
		return nil, err
	}

	APIURL, caCrt, clientCert, clientKey, err := cmd.ClusterUp(ctx, &rkeConfig, d.DockerDialer, nil, false, stateDir)
	if err != nil {
		return nil, err
	}

	if clusterInfo.Metadata == nil {
		clusterInfo.Metadata = map[string]string{}
	}

	clusterInfo.Metadata["Endpoint"] = APIURL
	clusterInfo.Metadata["RootCA"] = base64.StdEncoding.EncodeToString([]byte(caCrt))
	clusterInfo.Metadata["ClientCert"] = base64.StdEncoding.EncodeToString([]byte(clientCert))
	clusterInfo.Metadata["ClientKey"] = base64.StdEncoding.EncodeToString([]byte(clientKey))
	clusterInfo.Metadata["Config"] = yaml

	return d.save(clusterInfo, stateDir), nil
}

// PostCheck does post action
func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	info.Endpoint = info.Metadata["Endpoint"]
	info.ClientCertificate = info.Metadata["ClientCert"]
	info.ClientKey = info.Metadata["ClientKey"]
	info.RootCaCertificate = info.Metadata["RootCA"]

	certBytes, err := base64.StdEncoding.DecodeString(info.ClientCertificate)
	if err != nil {
		return nil, err
	}
	keyBytes, err := base64.StdEncoding.DecodeString(info.ClientKey)
	if err != nil {
		return nil, err
	}
	rootBytes, err := base64.StdEncoding.DecodeString(info.RootCaCertificate)
	if err != nil {
		return nil, err
	}

	host := info.Endpoint
	if !strings.HasPrefix(host, "https://") {
		host = fmt.Sprintf("https://%s", host)
	}
	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   rootBytes,
			CertData: certBytes,
			KeyData:  keyBytes,
		},
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes server version: %v", err)
	}

	token, err := drivers.GenerateServiceAccountToken(clientset)
	if err != nil {
		return nil, err
	}

	info.Version = serverVersion.GitVersion
	info.ServiceAccountToken = token

	info.NodeCount, err = nodeCount(info)
	return info, err
}

func nodeCount(info *types.ClusterInfo) (int64, error) {
	yaml, ok := info.Metadata["Config"]
	if !ok {
		return 0, nil
	}

	rkeConfig, err := drivers.ConvertToRkeConfig(yaml)
	if err != nil {
		return 0, err
	}

	count := int64(0)
	for _, node := range rkeConfig.Nodes {
		if slice.ContainsString(node.Role, "worker") {
			count++
		}
	}

	return count, nil
}

// Remove removes the cluster
func (d *Driver) Remove(ctx context.Context, clusterInfo *types.ClusterInfo) error {
	rkeConfig, err := drivers.ConvertToRkeConfig(clusterInfo.Metadata["Config"])
	if err != nil {
		return err
	}
	stateDir, _ := d.restore(clusterInfo)
	defer d.save(nil, stateDir)
	return cmd.ClusterRemove(ctx, &rkeConfig, d.DockerDialer, false, stateDir)
}

func (d *Driver) restore(info *types.ClusterInfo) (string, error) {
	os.MkdirAll(rancherPath, 0700)
	dir, err := ioutil.TempDir(rancherPath, "rke-")
	if err != nil {
		return "", err
	}

	if info != nil {
		state := info.Metadata["state"]
		if state != "" {
			ioutil.WriteFile(filepath.Join(dir, kubeConfigFile), []byte(state), 0600)
		}
	}

	return filepath.Join(dir, "cluster.yml"), nil
}

func (d *Driver) save(info *types.ClusterInfo, stateDir string) *types.ClusterInfo {
	if info != nil {
		b, err := ioutil.ReadFile(kubeConfig(stateDir))
		if err == nil {
			if info.Metadata == nil {
				info.Metadata = map[string]string{}
			}
			info.Metadata["state"] = string(b)
		}
	}

	if strings.HasPrefix(stateDir, rancherPath) && strings.HasSuffix(stateDir, "/cluster.yml") && !strings.Contains(stateDir, "..") {
		os.Remove(stateDir)
		os.Remove(kubeConfig(stateDir))
		os.Remove(filepath.Dir(stateDir))
	}

	return info
}

func kubeConfig(stateDir string) string {
	return filepath.Join(filepath.Dir(stateDir), kubeConfigFile)
}
