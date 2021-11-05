package rancherclient

import (
	projectClient "github.com/rancher/rancher/tests/automation-framework/client/generated/project/v3"
	"github.com/rancher/rancher/tests/automation-framework/clientbase"
	managementClient "github.com/rancher/rancher/tests/automation-framework/management"
	"github.com/rancher/rancher/tests/automation-framework/testsession"
	"k8s.io/client-go/rest"
)

type Client struct {
	Management    *managementClient.Client
	Project       *projectClient.Client
	RestConfig    *rest.Config
	RancherConfig *Config
}

// NewClient is returns a larger client wrapping individual api clients.
func NewClient(restConfig *rest.Config, testSession *testsession.TestSession) (*Client, error) {
	c := &Client{
		RestConfig: restConfig,
	}

	clientOpts := clientOpts(c.RestConfig, c.RancherConfig)

	managementClient, err := managementClient.NewClient(clientOpts, testSession)
	if err != nil {
		return nil, err
	}
	c.Management = managementClient

	projectClient, err := projectClient.NewClient(clientOpts)
	if err != nil {
		return nil, err
	}

	c.Project = projectClient
	c.Project.APIBaseClient.Ops.TestSession = testSession

	return c, nil
}

// NewRestConfig is the config used the various clients
func NewRestConfig(bearerToken string, rancherConfig *Config) *rest.Config {
	return &rest.Config{
		Host:        rancherConfig.RancherHost,
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: *rancherConfig.Insecure,
			CAFile:   rancherConfig.CAFile,
		},
	}
}

func clientOpts(restConfig *rest.Config, rancherConfig *Config) *clientbase.ClientOpts {
	return &clientbase.ClientOpts{
		URL:      restConfig.Host + "v3",
		TokenKey: restConfig.BearerToken,
		Insecure: restConfig.Insecure,
		CACerts:  rancherConfig.CACerts,
	}
}
