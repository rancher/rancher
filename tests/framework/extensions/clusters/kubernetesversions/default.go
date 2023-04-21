package kubernetesversions

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Default(t *testing.T, client *rancher.Client, provider string, kubernetesVersions []string) ([]string, error) {

	switch {
	case provider == clusters.RKE1ClusterType.String():
		default_version_data, err := client.Management.Setting.ByID("k8s-version")
		require.NoError(t, err)

		default_version := default_version_data.Value
		logrus.Infof("default rke1 kubernetes version is: %v", default_version)

		if kubernetesVersions == nil {
			kubernetesVersions = append(kubernetesVersions, default_version)
			logrus.Infof("no version found in kubernetesVersions; default rke1 kubernetes version %v will be used: %v", default_version, kubernetesVersions)
		}

		if kubernetesVersions[0] == "" {
			kubernetesVersions[0] = default_version
			logrus.Infof("empty string value found in kubernetesVersions; default rke1 kubernetes version %v will be used: %v", default_version, kubernetesVersions)
		}

	case provider == clusters.RKE2ClusterType.String():
		default_version_data, err := client.Management.Setting.ByID("rke2-default-version")
		require.NoError(t, err)

		default_version := `v` + default_version_data.Value
		logrus.Infof("default rke2 kubernetes version is: %v", default_version)

		if kubernetesVersions == nil {
			kubernetesVersions = append(kubernetesVersions, default_version)
			logrus.Infof("no version found in kubernetesVersions; default rke2 kubernetes version %v will be used: %v", default_version, kubernetesVersions)
		}

		if kubernetesVersions[0] == "" {
			kubernetesVersions[0] = default_version
			logrus.Infof("empty string value found in kubernetesVersions; default rke2 kubernetes version %v will be used: %v", default_version, kubernetesVersions)
		}

	case provider == clusters.K3SClusterType.String():
		default_version_data, err := client.Management.Setting.ByID("k3s-default-version")
		require.NoError(t, err)

		default_version := `v` + default_version_data.Value
		logrus.Infof("default k3s kubernetes version is: %v", default_version)

		if kubernetesVersions == nil {
			kubernetesVersions = append(kubernetesVersions, default_version)
			logrus.Infof("no version found in kubernetesVersions; default k3s kubernetes version %v will be used: %v", default_version, kubernetesVersions)
		}

		if kubernetesVersions[0] == "" {
			kubernetesVersions[0] = default_version
			logrus.Infof("empty string value found in kubernetesVersions; default k3s kubernetes version %v will be used: %v", default_version, kubernetesVersions)
		}

	default:
		return nil, fmt.Errorf("invalid provider: %v; valid providers: rke1, rke2, k3s", provider)
	}

	return kubernetesVersions, nil
}
