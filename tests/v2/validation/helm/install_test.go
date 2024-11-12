package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

func TestInstallHelmChart(t *testing.T) {
	settings := cli.New()
	
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(settings.RESTClientGetter(), "default", "", func(format string, v ...interface{}) {})
	require.NoError(t, err, "Failed to initialize action configuration")

	client := action.NewInstall(actionConfig)
	client.Namespace = "default"
	client.ReleaseName = "test-release"
	client.ChartPathOptions.RepoURL = "https://charts.helm.sh/stable"
	
	// Example using nginx chart
	chartName := "nginx"
	_, err = client.ChartPathOptions.LocateChart(chartName, settings)
	require.NoError(t, err, "Failed to locate chart")
}
