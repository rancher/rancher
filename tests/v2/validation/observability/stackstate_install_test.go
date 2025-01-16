//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/pkg/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type StackStateInstallTestSuite struct {
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

func (ssi *StackStateInstallTestSuite) TearDownSuite() {
	ssi.session.Cleanup()
}

func (ssi *StackStateInstallTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ssi.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(ssi.T(), err)

	ssi.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ssi.T(), clusterName, "Cluster name to install should be set")
	cluster, err := clusters.NewClusterMeta(ssi.client, clusterName)
	require.NoError(ssi.T(), err)
	ssi.cluster = cluster

	ssi.catalogClient, err = ssi.client.GetClusterCatalogClient(ssi.cluster.ID)
	require.NoError(ssi.T(), err)

	projectTemplate := kubeprojects.NewProjectTemplate(cluster.ID)
	projectTemplate.Name = charts.StackstateNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(ssi.T(), err)
	ssi.projectID = project.ID

	ssNamespaceExists, err := namespaces.GetNamespaceByName(client, cluster.ID, charts.StackstateNamespace)
	if ssNamespaceExists == nil && k8sErrors.IsNotFound(err) {
		_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackstateNamespace, "", map[string]string{}, map[string]string{})
	}
	require.NoError(ssi.T(), err)

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	ssi.stackstateConfigs = &stackstateConfigs

	// Install StackState Chart Repo
	_, err = ssi.catalogClient.ClusterRepos().Get(context.TODO(), observabilityChartName, meta.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		err = charts.CreateClusterRepo(ssi.client, ssi.catalogClient, observabilityChartName, observabilityChartURL)
		log.Info("Created suse-observability repo StackState install.")
	}
	require.NoError(ssi.T(), err)

	latestSSVersion, err := ssi.catalogClient.GetLatestChartVersion(charts.StackStateChartRepo, observabilityChartName)

	ssi.stackstateChartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: ssi.projectID,
	}
}

func (ssi *StackStateInstallTestSuite) TestInstallStackState() {
	subsession := ssi.session.NewSession()
	defer subsession.Cleanup()

	ssi.Run("Install SUSE Observability Chart", func() {

		// Read base config
		baseConfigData, err := os.ReadFile("resources/baseConfig_values.yaml")
		require.NoError(ssi.T(), err)

		var baseConfig observability.BaseConfig
		err = yaml.Unmarshal(baseConfigData, &baseConfig)
		require.NoError(ssi.T(), err)

		// Read sizing config
		sizingConfigData, err := os.ReadFile("resources/sizing_values.yaml")
		require.NoError(ssi.T(), err)

		var sizingConfig observability.SizingConfig
		err = yaml.Unmarshal(sizingConfigData, &sizingConfig)
		require.NoError(ssi.T(), err)

		// Convert structs back to map[string]interface{} for chart values
		baseConfigMap, err := structToMap(baseConfig)
		require.NoError(ssi.T(), err)

		sizingConfigMap, err := structToMap(sizingConfig)
		require.NoError(ssi.T(), err)

		// Merge the values
		mergedValues := mergeValues(baseConfigMap, sizingConfigMap)

		systemProject, err := projects.GetProjectByName(ssi.client, ssi.cluster.ID, systemProject)
		require.NoError(ssi.T(), err)
		require.NotNil(ssi.T(), systemProject.ID, "System project is nil.")
		systemProjectID := strings.Split(systemProject.ID, ":")[1]

		// Install the chart
		err = charts.InstallStackStateChart(ssi.client, ssi.stackstateChartInstallOptions, ssi.stackstateConfigs, systemProjectID, mergedValues)
		require.NoError(ssi.T(), err)
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

func TestStackStateInstallTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateInstallTestSuite))
}
