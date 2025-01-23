//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type StackStateServerTestSuite struct {
	suite.Suite
	client                        *rancher.Client
	session                       *session.Session
	cluster                       *clusters.ClusterMeta
	projectID                     string
	catalogClient                 *catalog.Client
	stackstateChartInstallOptions *charts.InstallOptions
	stackstateConfigs             *observability.StackStateConfig
}

const (
	observabilityChartURL  = "https://charts.rancher.com/server-charts/prime/suse-observability"
	observabilityChartName = "suse-observability"
)

func (sss *StackStateServerTestSuite) TearDownSuite() {
	sss.session.Cleanup()
}

func (sss *StackStateServerTestSuite) SetupSuite() {
	testSession := session.NewSession()
	sss.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(sss.T(), err)

	sss.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(sss.T(), clusterName, "Cluster name to install should be set")
	cluster, err := clusters.NewClusterMeta(sss.client, clusterName)
	require.NoError(sss.T(), err)
	sss.cluster = cluster

	sss.catalogClient, err = sss.client.GetClusterCatalogClient(sss.cluster.ID)
	require.NoError(sss.T(), err)

	projectTemplate := kubeprojects.NewProjectTemplate(cluster.ID)
	projectTemplate.Name = charts.StackstateNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(sss.T(), err)
	sss.projectID = project.ID

	ssNamespaceExists, err := namespaces.GetNamespaceByName(client, cluster.ID, charts.StackstateNamespace)
	if ssNamespaceExists == nil && k8sErrors.IsNotFound(err) {
		_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackstateNamespace, "", map[string]string{}, map[string]string{})
	}
	require.NoError(sss.T(), err)

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	sss.stackstateConfigs = &stackstateConfigs

	// Install StackState Chart Repo
	_, err = sss.catalogClient.ClusterRepos().Get(context.TODO(), observabilityChartName, meta.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		err = charts.CreateClusterRepo(sss.client, sss.catalogClient, observabilityChartName, observabilityChartURL)
		log.Info("Created suse-observability repo StackState install.")
	}
	require.NoError(sss.T(), err)

	latestSSVersion, err := sss.catalogClient.GetLatestChartVersion(charts.StackStateChartRepo, observabilityChartName)

	sss.stackstateChartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: sss.projectID,
	}
}

func (sss *StackStateServerTestSuite) TestInstallStackState() {
	subsession := sss.session.NewSession()
	defer subsession.Cleanup()

	sss.Run("Install SUSE Observability Chart", func() {
		// Read cattle config
		var stackstateConfigs observability.StackStateConfig
		config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)

		// Set base config values
		baseConfig := observability.BaseConfig{
			Global: struct {
				ImageRegistry string `json:"imageRegistry" yaml:"imageRegistry"`
			}{
				ImageRegistry: "registry.rancher.com",
			},
			Stackstate: observability.StackstateServerConfig{
				BaseUrl: stackstateConfigs.Url,
				Authentication: observability.AuthenticationConfig{
					AdminPassword: stackstateConfigs.AdminPassword,
				},
				ApiKey: observability.ApiKeyConfig{
					Key: stackstateConfigs.ClusterApiKey,
				},
				License: observability.LicenseConfig{
					Key: stackstateConfigs.License,
				},
			},
		}

		// Read sizing config
		sizingConfigData, err := os.ReadFile("resources/10-nonha_sizing_values.yaml")
		require.NoError(sss.T(), err)

		var sizingConfig observability.SizingConfig
		err = yaml.Unmarshal(sizingConfigData, &sizingConfig)
		require.NoError(sss.T(), err)

		// Convert structs back to map[string]interface{} for chart values
		baseConfigMap, err := structToMap(baseConfig)
		require.NoError(sss.T(), err)

		sizingConfigMap, err := structToMap(sizingConfig)
		require.NoError(sss.T(), err)

		// Merge the values
		mergedValues := mergeValues(baseConfigMap, sizingConfigMap)

		systemProject, err := projects.GetProjectByName(sss.client, sss.cluster.ID, systemProject)
		require.NoError(sss.T(), err)
		require.NotNil(sss.T(), systemProject.ID, "System project is nil.")
		systemProjectID := strings.Split(systemProject.ID, ":")[1]

		// Install the chart
		err = charts.InstallStackStateChart(sss.client, sss.stackstateChartInstallOptions, sss.stackstateConfigs, systemProjectID, mergedValues)
		require.NoError(sss.T(), err)
	})
}

// Helper function to convert struct to map[string]interface{}
func structToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	// First unmarshal into a map[interface{}]interface{}
	var m map[interface{}]interface{}
	err = yaml.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	// Convert to map[string]interface{}
	result := make(map[string]interface{})
	for k, v := range m {
		strKey := fmt.Sprintf("%v", k)
		result[strKey] = convertToStringKeysRecursive(v)
	}

	return result, nil
}

// convertToStringKeysRecursive recursively converts all map[interface{}]interface{} to map[string]interface{}
func convertToStringKeysRecursive(val interface{}) interface{} {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		strMap := make(map[string]interface{})
		for k, v2 := range v {
			strKey := fmt.Sprintf("%v", k)
			strMap[strKey] = convertToStringKeysRecursive(v2)
		}
		return strMap
	case []interface{}:
		for i, v2 := range v {
			v[i] = convertToStringKeysRecursive(v2)
		}
		return v
	default:
		return v
	}
}

// mergeValues merges multiple YAML values maps into a single map
func mergeValues(values ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, v := range values {
		for key, value := range v {
			if existingValue, ok := result[key]; ok {
				if existingMap, ok := existingValue.(map[string]interface{}); ok {
					if newMap, ok := value.(map[string]interface{}); ok {
						// Recursively merge maps
						result[key] = mergeValues(existingMap, newMap)
						continue
					}
				}
			}
			result[key] = value
		}
	}
	return result
}

func TestStackStateServerTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateServerTestSuite))
}
