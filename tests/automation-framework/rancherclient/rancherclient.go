package rancherclient

import (
	"fmt"

	"github.com/rancher/rancher/tests/automation-framework/clientbase"
	managementClient "github.com/rancher/rancher/tests/automation-framework/management"
	"github.com/rancher/rancher/tests/automation-framework/testsession"
	"k8s.io/client-go/rest"
)

type Client struct {
	Management    *managementClient.Client
	RancherConfig *Config
}

// NewClient is returns a larger client wrapping individual api clients.
func NewClient(bearerToken string, rancherConfig *Config, testSession *testsession.TestSession) (*Client, error) {
	c := &Client{
		RancherConfig: rancherConfig,
	}

	var err error
	c.Management, err = managementClient.NewClient(clientOpts(newRestConfig(bearerToken, rancherConfig), c.RancherConfig), testSession)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func newRestConfig(bearerToken string, rancherConfig *Config) *rest.Config {
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
		URL:      fmt.Sprintf("https://%s/v3", rancherConfig.RancherHost),
		TokenKey: restConfig.BearerToken,
		Insecure: restConfig.Insecure,
		CACerts:  rancherConfig.CACerts,
	}
}
