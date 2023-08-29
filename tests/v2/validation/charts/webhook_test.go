package charts

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"

	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type WebhookTestSuite struct {
	suite.Suite
	client       *rancher.Client
	session      *session.Session
	clusterList  string
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
	w.clusterList = client.RancherConfig.ClusterName
	w.chartVersion, err = client.Catalog.GetLatestChartVersion(charts.RancherWebhookName)
	require.NoError(w.T(), err)

}

func (w *WebhookTestSuite) ValidateEscalationCheck(client *rancher.Client) {

	getAdminRole, err := client.Management.GlobalRole.ByID(admin)
	require.NoError(w.T(), err)
	updatedAdminRole := *getAdminRole
	updatedAdminRole.NewUserDefault = true

	_, err = client.Management.GlobalRole.Update(getAdminRole, updatedAdminRole)
	require.Error(w.T(), err)
	errMessage := "admission webhook \"rancher.cattle.io.globalroles.management.cattle.io\" denied the request"

	assert.Contains(w.T(), err.Error(), errMessage)
}

func (w *WebhookTestSuite) ValidateWebhookPodLogs(podName, clusterID string) {

	podLogs, err := kubeconfig.GetPodLogs(w.client, clusterID, podName, charts.RancherWebhookNamespace)
	require.NoError(w.T(), err)
	delimiter := "\n"
	segments := strings.Split(podLogs, delimiter)

	for _, segment := range segments {
		if strings.Contains(segment, "level=error") {
			w.Fail("Error logs in webhook", segment)
		}
	}

}

func (w *WebhookTestSuite) TestWebhookChart() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	clusterID, err := clusters.GetClusterIDByName(w.client, w.clusterList)
	require.NoError(w.T(), err)

	w.Run("Verify the version of webhook on local and downstream cluster", func() {
		subSession := w.session.NewSession()
		defer subSession.Cleanup()

		initialWebhookChart, err := charts.GetChartStatus(w.client, clusterID, charts.RancherWebhookNamespace, charts.RancherWebhookName)
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
		w.ValidateWebhookPodLogs(podName, clusterID)

	})

	w.Run("Verify the count of webhook is greater than zero and list webhooks", func() {
		webhookList, err := getWebhookNames(w.client, clusterID, resourceName)
		require.NoError(w.T(), err)

		assert.True(w.T(), len(webhookList) > 0, "Expected webhooks list to be greater than zero")
		log.Info("Count of webhook obtained for the cluster: ", w.clusterList, "is ", len(webhookList))
		listStr := strings.Join(webhookList, ", ")
		log.WithField("", listStr).Info("List of webhooks obtained for the ", w.clusterList)

	})

	w.Run("Verify escalation check", func() {
		newUser, err := users.CreateUserWithRole(w.client, users.UserConfig(), restrictedAdmin)
		require.NoError(w.T(), err)
		w.T().Logf("Created user: %v", newUser.Name)

		restrictedAdminClient, err := w.client.AsUser(newUser)
		require.NoError(w.T(), err)

		w.ValidateEscalationCheck(restrictedAdminClient)
	})

}

func TestWebhookTestSuite(t *testing.T) {
	suite.Run(t, new(WebhookTestSuite))
}
