package rke

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/kontainer-engine/drivers/rke/rkecerts"
	"github.com/rancher/kontainer-engine/drivers/util"
	"github.com/rancher/kontainer-engine/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rke/cmd"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	kubeConfigFile = "kube_config_cluster.yml"
	rancherPath    = "./management-state/rke/"
)

// Driver is the struct of rke driver

type WrapTransportFactory func(config *v3.RancherKubernetesEngineConfig) k8s.WrapTransport

type Driver struct {
	DockerDialer         hosts.DialerFactory
	LocalDialer          hosts.DialerFactory
	WrapTransportFactory WrapTransportFactory
	driverCapabilities   types.Capabilities

	types.UnimplementedVersionAccess
	types.UnimplementedClusterSizeAccess
}

func NewDriver() types.Driver {
	d := &Driver{
		driverCapabilities: types.Capabilities{
			Capabilities: make(map[int64]bool),
		},
	}

	d.driverCapabilities.AddCapability(types.GetVersionCapability)
	d.driverCapabilities.AddCapability(types.SetVersionCapability)

	d.driverCapabilities.AddCapability(types.GetClusterSizeCapability)

	return d
}

func (d *Driver) wrapTransport(config *v3.RancherKubernetesEngineConfig) k8s.WrapTransport {
	if d.WrapTransportFactory == nil {
		return nil
	}

	return k8s.WrapTransport(func(rt http.RoundTripper) http.RoundTripper {
		fn := d.WrapTransportFactory(config)
		if fn == nil {
			return rt
		}
		return fn(rt)
	})

}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
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

	rkeConfig, err := util.ConvertToRkeConfig(yaml)
	if err != nil {
		return nil, err
	}

	stateDir, err := d.restore(nil)
	if err != nil {
		return nil, err
	}
	defer d.cleanup(stateDir)

	certsStr := ""
	APIURL, caCrt, clientCert, clientKey, certs, err := cmd.ClusterUp(ctx, &rkeConfig, d.DockerDialer, d.LocalDialer,
		d.wrapTransport(&rkeConfig), false, stateDir, false, false)
	if err == nil {
		certsStr, err = rkecerts.ToString(certs)
	}
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
			"Certs":      certsStr,
		},
	}, stateDir), nil
}

// Update updates the rke cluster
func (d *Driver) Update(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	yaml, err := getYAML(opts)
	if err != nil {
		return nil, err
	}

	rkeConfig, err := util.ConvertToRkeConfig(yaml)
	if err != nil {
		return nil, err
	}

	stateDir, err := d.restore(clusterInfo)
	if err != nil {
		return nil, err
	}
	defer d.cleanup(stateDir)

	certStr := ""
	APIURL, caCrt, clientCert, clientKey, certs, err := cmd.ClusterUp(ctx, &rkeConfig, d.DockerDialer, d.LocalDialer,
		d.wrapTransport(&rkeConfig), false, stateDir, false, false)
	if err == nil {
		certStr, err = rkecerts.ToString(certs)
	}
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
	clusterInfo.Metadata["Certs"] = certStr

	return d.save(clusterInfo, stateDir), nil
}

func (d *Driver) getClientset(info *types.ClusterInfo) (*kubernetes.Clientset, error) {
	yaml := info.Metadata["Config"]

	rkeConfig, err := util.ConvertToRkeConfig(yaml)
	if err != nil {
		return nil, err
	}

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
		WrapTransport: d.WrapTransportFactory(&rkeConfig),
	}

	return kubernetes.NewForConfig(config)
}

// PostCheck does post action
func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	clientset, err := d.getClientset(info)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		serverVersion, err := clientset.DiscoveryClient.ServerVersion()
		if err != nil {
			lastErr = fmt.Errorf("failed to get Kubernetes server version: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		token, err := util.GenerateServiceAccountToken(clientset)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}

		info.Version = serverVersion.GitVersion
		info.ServiceAccountToken = token

		info.NodeCount, err = nodeCount(info)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}

		return info, err
	}

	return nil, lastErr
}

func (d *Driver) GetVersion(ctx context.Context, info *types.ClusterInfo) (*types.KubernetesVersion, error) {
	clientset, err := d.getClientset(info)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	serviceVersion, err := clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %v", err)
	}

	return &types.KubernetesVersion{Version: serviceVersion.String()}, nil
}

func (d *Driver) SetVersion(ctx context.Context, info *types.ClusterInfo, version *types.KubernetesVersion) error {
	config, err := util.ConvertToRkeConfig(info.Metadata["Config"])

	if err != nil {
		return err
	}

	config.Version = version.Version

	stateDir, err := d.restore(info)
	if err != nil {
		return err
	}
	defer d.cleanup(stateDir)

	_, _, _, _, _, err = cmd.ClusterUp(ctx, &config, d.DockerDialer, d.LocalDialer,
		d.wrapTransport(&config), false, stateDir, false, false)

	if err != nil {
		return err
	}

	d.save(info, stateDir)

	return nil
}

func (d *Driver) GetClusterSize(ctx context.Context, info *types.ClusterInfo) (*types.NodeCount, error) {
	clientset, err := d.getClientset(info)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	nodeList, err := clientset.CoreV1().Nodes().List(v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %v", err)
	}

	return &types.NodeCount{Count: int64(len(nodeList.Items))}, nil
}

func nodeCount(info *types.ClusterInfo) (int64, error) {
	yaml, ok := info.Metadata["Config"]
	if !ok {
		return 0, nil
	}

	rkeConfig, err := util.ConvertToRkeConfig(yaml)
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
	rkeConfig, err := util.ConvertToRkeConfig(clusterInfo.Metadata["Config"])
	if err != nil {
		return err
	}
	stateDir, _ := d.restore(clusterInfo)
	defer d.save(nil, stateDir)
	return cmd.ClusterRemove(ctx, &rkeConfig, d.DockerDialer, d.wrapTransport(&rkeConfig), false, stateDir)
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
			ioutil.WriteFile(kubeConfig(dir), []byte(state), 0600)
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

	d.cleanup(stateDir)

	return info
}

func (d *Driver) cleanup(stateDir string) {
	if strings.HasSuffix(stateDir, "/cluster.yml") && !strings.Contains(stateDir, "..") {
		os.Remove(stateDir)
		os.Remove(kubeConfig(stateDir))
		os.Remove(filepath.Dir(stateDir))
	}
}

func kubeConfig(stateDir string) string {
	if strings.HasSuffix(stateDir, "/cluster.yml") {
		return filepath.Join(filepath.Dir(stateDir), kubeConfigFile)
	}
	return filepath.Join(stateDir, kubeConfigFile)
}
