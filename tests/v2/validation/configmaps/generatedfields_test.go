//go:build (infra.any || cluster.any || sanity || validation) && !stress && !extended

package configmaps

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	cm "github.com/rancher/shepherd/extensions/configmaps"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const configMapNamespace = "default"

type ConfigMapTestSuite struct {
	suite.Suite
	client             *rancher.Client
	steveClient        *steveV1.Client
	session            *session.Session
	cluster            *management.Cluster
	nameSpacedV1Client *steveV1.NamespacedSteveClient
	configMapPayload   *cm.SteveConfigMap
}

func (c *ConfigMapTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *ConfigMapTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession
	client, err := rancher.NewClient("", c.session)
	require.NoError(c.T(), err)
	c.client = client
	c.steveClient = client.Steve
}

func (c *ConfigMapTestSuite) TestSteveGeneratedFields() {

	steveClient := c.client.Steve
	configMapName := namegenerator.AppendRandomString("test-configmap")

	v1ConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
		},
		Data: map[string]string{
			"env": "qa",
		},
	}

	configmapSteveObject, err := steveClient.SteveType(cm.ConfigMapSteveType).Create(v1ConfigMap)
	require.NoError(c.T(), err)

	c.configMapPayload = new(cm.SteveConfigMap)
	err = setPayload(configmapSteveObject, c.configMapPayload)
	require.NoError(c.T(), err)

	headers, _, err := steveClient.SteveType("configmaps").NamespacedSteveClient(configMapNamespace).PerformPutCaptureHeaders(c.client.RancherConfig.Host, c.client.RancherConfig.AdminToken, c.configMapPayload.Name, c.configMapPayload)
	require.NoError(c.T(), err)

	warnings, ok := headers["Warning"]
	var failWarnings []string
	if ok {
		for _, warning := range warnings {
			if strings.HasPrefix(warning, "299") {
				logrus.Printf("Warning header found: %s", warning)
				failWarnings = append(failWarnings, warning)
			}
		}
	}

	if len(failWarnings) > 0 {
		require.Fail(c.T(), fmt.Sprintf("Test failed due to warnings: \n%s", strings.Join(failWarnings, "\n")))
	}
}

func TestConfigMapTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigMapTestSuite))
}
