//go:build (validation || infra.any || cluster.any || stress) && !sanity && !extended

package charts

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/extensions/workloads/pods"

	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type WebhookTestSuite struct {
	suite.Suite
	client       *rancher.Client
	session      *session.Session
	clusterName  string
	chartVersion string
}

func (w *WebhookTestSuite) TearDownSuite() {
	w.session.Cleanup()
}

func (w *WebhookTestSuite) SetupSuite() {
	testSession := session.NewSession()
	w.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(w.T(), err)

	w.client = client

	// Get clusterName from config yaml
	w.clusterName = client.RancherConfig.ClusterName
	w.chartVersion, err = client.Catalog.GetLatestChartVersion(charts.RancherWebhookName, catalog.RancherChartRepo)
	require.NoError(w.T(), err)
}

func (w *WebhookTestSuite) TestWebhookChart() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		cluster string
	}{
		{localCluster},
		{w.clusterName},
	}

	for _, tt := range tests {

		clusterID, err := clusters.GetClusterIDByName(w.client, tt.cluster)
		require.NoError(w.T(), err)

		w.Run("Verify the version of webhook on "+tt.cluster, func() {
			subSession := w.session.NewSession()
			defer subSession.Cleanup()

			initialWebhookChart, err := extencharts.GetChartStatus(w.client, clusterID, charts.RancherWebhookNamespace, charts.RancherWebhookName)
			require.NoError(w.T(), err)
			chartVersion := initialWebhookChart.ChartDetails.Spec.Chart.Metadata.Version
			require.NoError(w.T(), err)
			assert.Equal(w.T(), w.chartVersion, chartVersion)
		})

		w.Run("Verify webhook pod logs", func() {
			steveClient, err := w.client.Steve.ProxyDownstream(clusterID)
			require.NoError(w.T(), err)

			pods, err := steveClient.SteveType(pods.PodResourceSteveType).NamespacedSteveClient(charts.RancherWebhookNamespace).List(nil)
			require.NoError(w.T(), err)

			var podName string
			for _, pod := range pods.Data {
				if strings.Contains(pod.Name, charts.RancherWebhookName) {
					podName = pod.Name
				}
			}

			podLogs, err := kubeconfig.GetPodLogs(w.client, clusterID, podName, charts.RancherWebhookNamespace, "")
			require.NoError(w.T(), err)
			webhookLogs := validateWebhookPodLogs(podLogs)
			require.Nil(w.T(), webhookLogs)
		})

		w.Run("Verify the count of webhook is greater than zero and list webhooks", func() {
			webhookList, err := getWebhookNames(w.client, clusterID, resourceName)
			require.NoError(w.T(), err)

			assert.True(w.T(), len(webhookList) > 0, "Expected webhooks list to be greater than zero")
			log.Info("Count of webhook obtained for the cluster: ", tt.cluster, " is ", len(webhookList))
			listStr := strings.Join(webhookList, ", ")
			log.WithField("", listStr).Info("List of webhooks obtained for the ", tt.cluster)
		})
	}
}

func (w *WebhookTestSuite) TestWebhookEscalationCheck() {
	w.Run("Verify escalation check", func() {
		newUser, err := users.CreateUserWithRole(w.client, users.UserConfig(), restrictedAdmin)
		require.NoError(w.T(), err)
		w.T().Logf("Created user: %v", newUser.Name)

		restrictedAdminClient, err := w.client.AsUser(newUser)
		require.NoError(w.T(), err)

		getAdminRole, err := restrictedAdminClient.Management.GlobalRole.ByID(admin)
		require.NoError(w.T(), err)
		updatedAdminRole := *getAdminRole
		updatedAdminRole.NewUserDefault = true

		_, err = restrictedAdminClient.Management.GlobalRole.Update(getAdminRole, updatedAdminRole)
		require.Error(w.T(), err)
		errMessage := "admission webhook \"rancher.cattle.io.globalroles.management.cattle.io\" denied the request"
		assert.Contains(w.T(), err.Error(), errMessage)
	})
}

func TestWebhookTestSuite(t *testing.T) {
	suite.Run(t, new(WebhookTestSuite))
}
