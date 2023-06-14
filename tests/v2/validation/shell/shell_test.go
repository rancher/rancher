//go:build (validation || infra.any || cluster.any) && !stress && !sanity && !extended

package shell

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"

	"github.com/rancher/rancher/tests/framework/pkg/session"

	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/settings"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

const (
	cattleSystemNameSpace = "cattle-system"
	shellName             = "shell-image"
)

type ShellTestSuite struct {
	suite.Suite
	client      *rancher.Client
	session     *session.Session
	clusterName string
}

func (s *ShellTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *ShellTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client

	// Get clusterName from config yaml
	s.clusterName = client.RancherConfig.ClusterName
	require.NoError(s.T(), err)

}

func (s *ShellTestSuite) TestShell() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	s.Run("Verify the version of shell on local cluster", func() {
		shellImage, err := settings.ShellVersion(s.client, s.clusterName, shellName)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), shellImage, s.client.RancherConfig.ShellImage)
	})

	s.Run("Verify the helm operations for the shell succeeded", func() {
		steveClient := s.client.Steve
		pods, err := steveClient.SteveType(pods.PodResourceSteveType).NamespacedSteveClient(cattleSystemNameSpace).List(nil)
		require.NoError(s.T(), err)

		for _, pod := range pods.Data {
			if strings.Contains(pod.Name, "helm") {
				podStatus := &corev1.PodStatus{}
				err = steveV1.ConvertToK8sType(pod.Status, podStatus)
				require.NoError(s.T(), err)
				assert.Equal(s.T(), "Succeeded", string(podStatus.Phase))
			}
		}
	})
}

func TestShellTestSuite(t *testing.T) {
	suite.Run(t, new(ShellTestSuite))
}
