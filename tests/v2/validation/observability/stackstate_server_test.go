//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
	// Standard library imports
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	// Third-party library imports
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Local/internal imports
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	rancherProjects "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
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
	projectTemplate.Name = charts.StackStateServerNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(sss.T(), err)
	sss.projectID = project.ID

	ssNamespaceExists, err := namespaces.GetNamespaceByName(client, cluster.ID, charts.StackStateServerNamespace)
	if ssNamespaceExists == nil && k8sErrors.IsNotFound(err) {
		_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackStateServerNamespace, "", map[string]string{}, map[string]string{})
	}
	require.NoError(sss.T(), err)

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	sss.stackstateConfigs = &stackstateConfigs

	_, err = sss.catalogClient.ClusterRepos().Get(context.TODO(), observabilityChartName, meta.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		err = charts.CreateClusterRepo(sss.client, sss.catalogClient, observabilityChartName, observabilityChartURL)
		log.Info("Created suse-observability repo StackState install.")
	}
	require.NoError(sss.T(), err)

	latestSSVersion, err := sss.catalogClient.GetLatestChartVersion(charts.StackStateServerChartRepo, observabilityChartName)

	sss.stackstateChartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: sss.projectID,
	}
}

func (sss *StackStateServerTestSuite) TestInstallStackState() {
	subsession := sss.session.NewSession()
	defer subsession.Cleanup()

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)

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

	ingressConfig := observability.IngressConfig{
		Ingress: observability.Ingress{
			Enabled: true,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/proxy-body-size": "50m",
			},
			Hosts: []observability.Host{
				{
					Host: stackstateConfigs.Url,
				},
			},
			TLS: []observability.TLSConfig{
				{
					Hosts:      []string{stackstateConfigs.Url},
					SecretName: "tls-secret",
				},
			},
		},
	}

	sizingConfigData, err := os.ReadFile("resources/10-nonha_sizing_values.yaml")
	require.NoError(sss.T(), err)

	var sizingConfig observability.SizingConfig
	err = yaml.Unmarshal(sizingConfigData, &sizingConfig)
	require.NoError(sss.T(), err)

	ingressConfigMap, err := structToMap(ingressConfig)
	require.NoError(sss.T(), err)

	baseConfigMap, err := structToMap(baseConfig)
	require.NoError(sss.T(), err)

	sizingConfigMap, err := structToMap(sizingConfig)
	require.NoError(sss.T(), err)

	mergedValues := mergeValues(ingressConfigMap, baseConfigMap, sizingConfigMap)

	systemProject, err := rancherProjects.GetProjectByName(sss.client, sss.cluster.ID, systemProject)
	require.NoError(sss.T(), err)
	require.NotNil(sss.T(), systemProject.ID, "System project is nil.")
	systemProjectID := strings.Split(systemProject.ID, ":")[1]

	sss.Run("Install SUSE Observability Server Chart with non HA values", func() {
		err = charts.InstallStackStateServerChart(sss.client, sss.stackstateChartInstallOptions, systemProjectID, mergedValues)
		require.NoError(sss.T(), err)
		log.Info("Stackstate server chart installed successfully")

		sss.T().Log("Verifying the deployments of stackstate server chart to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(sss.client, sss.cluster.ID, charts.StackStateServerNamespace, meta.ListOptions{})
		require.NoError(sss.T(), err)

		sss.T().Log("Verifying the daemonsets of stackstate server chart to have expected number of available replicas nodes")
		err = extencharts.WatchAndWaitDaemonSets(sss.client, sss.cluster.ID, charts.StackStateServerNamespace, meta.ListOptions{})
		require.NoError(sss.T(), err)

		clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(sss.client, sss.client.RancherConfig.ClusterName, fleet.Namespace)
		var clusterName string
		if clusterObject != nil {
			status := &provv1.ClusterStatus{}
			err := steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(sss.T(), err)
			clusterName = status.ClusterName
		} else {
			clusterName, err = extensionscluster.GetClusterIDByName(sss.client, sss.client.RancherConfig.ClusterName)
			require.NoError(sss.T(), err)
		}
		podErrors := pods.StatusPods(sss.client, clusterName)
		require.Empty(sss.T(), podErrors)
	})
}

// Helper function to convert struct to map[string]interface{}
func structToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var intermediate map[interface{}]interface{}
	if err = yaml.Unmarshal(data, &intermediate); err != nil {
		return nil, err
	}

	return convertMapInterfaceToMapString(intermediate), nil
}

// Converts map[interface{}]interface{} to map[string]interface{}
func convertMapInterfaceToMapString(obj map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range obj {
		key := fmt.Sprintf("%v", k)
		result[key] = convertToStringKeys(v)
	}
	return result
}

// convertToStringKeys converts any map[interface{}]interface{} to map[string]interface{} recursively
func convertToStringKeys(val interface{}) interface{} {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		return convertMapToStringKeys(v)
	case []interface{}:
		convertedSlice := make([]interface{}, len(v))
		for i, v2 := range v {
			convertedSlice[i] = convertToStringKeys(v2)
		}
		return convertedSlice
	default:
		return v
	}
}

// convertMapToStringKeys converts a map[interface{}]interface{} to a map[string]interface{}
func convertMapToStringKeys(input map[interface{}]interface{}) map[string]interface{} {
	strMap := make(map[string]interface{})
	for k, v := range input {
		strKey := fmt.Sprintf("%v", k)
		strMap[strKey] = convertToStringKeys(v)
	}
	return strMap
}

// mergeValues merges multiple map[string]interface{} values into one, recursively merging nested maps if keys overlap.
func mergeValues(values ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, currentMap := range values {
		for key, currentValue := range currentMap {
			if existingValue, exists := result[key]; exists {
				// Check if both existing and current values are maps,
				// if so, recursively merge them
				mergedMap := mergeMaps(existingValue, currentValue)
				if mergedMap != nil {
					result[key] = mergedMap
					continue
				}
			}
			// Otherwise, overwrite with the current value
			result[key] = currentValue
		}
	}
	return result
}

// mergeMaps recursively merges two maps if both are maps, else returns nil
func mergeMaps(existingValue, currentValue interface{}) map[string]interface{} {
	existingMap, ok1 := existingValue.(map[string]interface{})
	currentMap, ok2 := currentValue.(map[string]interface{})
	if ok1 && ok2 {
		return mergeValues(existingMap, currentMap)
	}
	return nil
}

func TestStackStateServerTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateServerTestSuite))
}
