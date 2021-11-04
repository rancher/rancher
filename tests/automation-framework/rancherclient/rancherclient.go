package rancherclient

import (
	projectClient "github.com/rancher/rancher/tests/automation-framework/client/generated/project/v3"
	"github.com/rancher/rancher/tests/automation-framework/clientbase"
	"github.com/rancher/rancher/tests/automation-framework/config"
	managementClient "github.com/rancher/rancher/tests/automation-framework/management"
	"github.com/rancher/rancher/tests/automation-framework/testsession"
	"k8s.io/client-go/rest"
)

type RancherClient struct {
	Management    *managementClient.Client
	Project       *projectClient.Client
	RestConfig    *rest.Config
	RancherConfig *config.RancherServerConfig
}

// NewClient is returns a larger client wrapping individual api clients.
func NewClient(restConfig *rest.Config, testSession *testsession.TestSession) (*RancherClient, error) {
	c := &RancherClient{
		RestConfig: restConfig,
	}

	err := c.newManagementClient(testSession)
	if err != nil {
		return nil, err
	}

	err = c.newProjectClient()
	if err != nil {
		return nil, err
	}
	c.Project.APIBaseClient.Ops.TestSession = testSession

	return c, nil
}

// NewRestConfig is the config used the various clients
func NewRestConfig(bearerToken string, rancherConfig *config.RancherServerConfig) *rest.Config {
	return &rest.Config{
		Host:        rancherConfig.GetCattleTestURL(),
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: rancherConfig.GetInsecure(),
			CAFile:   rancherConfig.GetCAFile(),
		},
	}
}

func clientOpts(restConfig *rest.Config, rancherConfig *config.RancherServerConfig) *clientbase.ClientOpts {
	return &clientbase.ClientOpts{
		URL:      restConfig.Host + "v3",
		TokenKey: restConfig.BearerToken,
		Insecure: restConfig.Insecure,
		CACerts:  rancherConfig.GetCAFile(),
	}
}

func (c *RancherClient) newManagementClient(testSession *testsession.TestSession) error {
	clientOpts := clientOpts(c.RestConfig, c.RancherConfig)
	client, err := managementClient.NewClient(clientOpts, testSession)
	if err != nil {
		return err
	}
	c.Management = client
	return nil
}

func (c *RancherClient) newProjectClient() error {
	clientOpts := clientOpts(c.RestConfig, c.RancherConfig)
	client, err := projectClient.NewClient(clientOpts)
	if err != nil {
		return err
	}

	c.Project = client
	return nil
}
