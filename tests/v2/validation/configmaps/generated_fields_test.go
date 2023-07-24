package configmaps

import (
	"fmt"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	cm "github.com/rancher/rancher/tests/framework/extensions/configmaps"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/configmaps"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	"strings"
	"testing"
)

const (
	APIVersion         = "v1"
	Kind               = "ConfigMap"
	configMapsEndpoint = "configmaps"
	configMapNamespace = "default"
)

type ConfigMapTestSuite struct {
	suite.Suite
	client             *rancher.Client
	steveClient        *steveV1.Client
	session            *session.Session
	cluster            *management.Cluster
	nameSpacedV1Client *steveV1.NamespacedSteveClient
	configMapPayload   *cm.GenFieldTest
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

	configData := map[string]string{
		"env": "qa",
	}
	labels, annotations := map[string]string{}, map[string]string{}
	configMapName := namegenerator.AppendRandomString("test-configmap")
	configmap, err := configmaps.CreateConfigMap(c.client, c.client.RancherConfig.ClusterName, configMapName, "auto-generated", configMapNamespace, configData, labels, annotations)
	require.NoError(c.T(), err)

	logrus.Infof("created a configmap(%v, configmap).............", configmap.Name)

	c.configMapPayload = &cm.GenFieldTest{}
	copyConfigMapToPayload(configmap, c.configMapPayload)

	// Update the configmap payload with the fields that we want to test.
	c.configMapPayload.Metadata.Fields = "test-fields"
	c.configMapPayload.Metadata.Relationships = "test-relationships"
	c.configMapPayload.Metadata.State = "test-state"
	c.configMapPayload.Data["foo"] = "bar"

	headers, _, err := c.nameSpacedV1Client.PerformPutCaptureHeaders(c.client.RancherConfig.Host, c.client.RancherConfig.AdminToken, configMapsEndpoint, configmap.Namespace, configmap.Name, c.configMapPayload)
	require.NoError(c.T(), err)

	// Check for warnings in the response headers.
	warnings, ok := headers["Warning"]
	var failWarnings []string
	if ok {
		for _, warning := range warnings {
			// If the warning message starts with "299", then log it and add it to failWarnings.
			if strings.HasPrefix(warning, "299") {
				logrus.Printf("Warning header found: %s", warning)
				failWarnings = append(failWarnings, warning)
			}
		}
	}

	// If there were any "299" warnings, fail the test.
	if len(failWarnings) > 0 {
		require.Fail(c.T(), fmt.Sprintf("Test failed due to warnings: \n%s", strings.Join(failWarnings, "\n")))
	}
}

func TestConfigMapTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigMapTestSuite))
}

func copyConfigMapToPayload(cm *v1.ConfigMap, payload *cm.GenFieldTest) {
	payload.APIVersion = APIVersion
	payload.Data = cm.Data
	payload.Kind = Kind
	payload.Metadata.Name = cm.ObjectMeta.Name
	payload.Metadata.Namespace = cm.ObjectMeta.Namespace
	payload.Metadata.ResourceVersion = cm.ObjectMeta.ResourceVersion
	payload.Metadata.UID = string(cm.ObjectMeta.UID)
}
