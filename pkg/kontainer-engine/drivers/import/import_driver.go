package kubeimport

import (
	"context"

	"io/ioutil"

	"encoding/base64"

	"fmt"

	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Driver struct {
	types.Capabilities

	types.UnimplementedVersionAccess
	types.UnimplementedClusterSizeAccess
}

func NewDriver() types.Driver {
	return &Driver{}
}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &types.Capabilities{Capabilities: make(map[int64]bool)}, nil
}

func (d *Driver) GetK8SCapabilities(ctx context.Context, opts *types.DriverOptions) (*types.K8SCapabilities, error) {
	return &types.K8SCapabilities{}, nil
}

func getDriverOptions() *types.DriverFlags {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["kubeConfig"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the contents of the kubeconfig file",
	}
	driverFlag.Options["kubeConfigPath"] = &types.Flag{
		Type:  types.StringType,
		Usage: "the path to the kubeconfig file",
	}
	return &driverFlag
}

func (d *Driver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	return getDriverOptions(), nil
}

func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	return getDriverOptions(), nil
}

func (d *Driver) Create(ctx context.Context, opts *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Info("importing kubeconfig file into clusters")

	configPath := opts.StringOptions["kubeConfigPath"]

	var raw []byte
	var err error

	if configPath == "" {
		raw = []byte(opts.StringOptions["kubeConfig"])
	} else {
		raw, err = ioutil.ReadFile(configPath)

		if err != nil {
			return nil, fmt.Errorf("failed to open kubeconfig file: %v", err)
		}
	}

	clusters := &store.KubeConfig{}
	err = yaml.Unmarshal(raw, clusters)

	if err != nil {
		return nil, fmt.Errorf("error unmarshalling kubeconfig: %v", err)
	}

	if len(clusters.Clusters) == 0 {
		return nil, fmt.Errorf("kubeconfig has no clusters")
	}

	if len(clusters.Contexts) == 0 {
		return nil, fmt.Errorf("kubeconfig has no contexts")
	}

	if len(clusters.Users) == 0 {
		return nil, fmt.Errorf("kubeconfig has no users")
	}

	info := &types.ClusterInfo{}
	info.Endpoint = clusters.Clusters[0].Cluster.Server
	info.Username = clusters.Contexts[0].Context.User

	info.RootCaCertificate = clusters.Clusters[0].Cluster.CertificateAuthorityData
	info.ClientCertificate = clusters.Users[0].User.ClientCertificateData
	info.ClientKey = clusters.Users[0].User.ClientKeyData

	info.Username = clusters.Users[0].User.Username
	info.Password = clusters.Users[0].User.Password

	info.Metadata = make(map[string]string)
	info.Metadata["Endpoint"] = clusters.Clusters[0].Cluster.Server
	info.Metadata["Config"] = string(raw)

	info.Metadata["RootCA"] = info.RootCaCertificate
	info.Metadata["ClientCert"] = info.ClientCertificate
	info.Metadata["ClientKey"] = info.ClientKey

	return info, nil
}

func (d *Driver) Update(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Info("starting post check")

	capem, err := base64.StdEncoding.DecodeString(info.RootCaCertificate)

	if err != nil {
		return nil, fmt.Errorf("error decoding root ca certificate: %v", err)
	}

	key, err := base64.StdEncoding.DecodeString(info.ClientKey)

	if err != nil {
		return nil, fmt.Errorf("error decoding client key: %v", err)
	}

	cert, err := base64.StdEncoding.DecodeString(info.ClientCertificate)

	if err != nil {
		return nil, fmt.Errorf("error decoding client certificate: %v", err)
	}

	config := &rest.Config{
		Host: info.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   capem,
			KeyData:  key,
			CertData: cert,
		},
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	_, err = clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes server version: %v", err)
	}

	info.ServiceAccountToken, err = util.GenerateServiceAccountToken(clientset)

	if err != nil {
		return nil, err
	}

	logrus.Info("service account token generated successfully")
	logrus.Info("post-check completed successfully")

	return info, nil
}

func (d *Driver) Remove(ctx context.Context, clusterInfo *types.ClusterInfo) error {
	// Nothing to do
	return nil
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

func (d *Driver) RemoveLegacyServiceAccount(ctx context.Context, info *types.ClusterInfo) error {
	return fmt.Errorf("not implemented")
}
