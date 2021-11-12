package rancher

import (
	"fmt"

	frameworkDynamic "github.com/rancher/rancher/tests/framework/clients/dynamic"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	provisioning "github.com/rancher/rancher/tests/framework/clients/rancher/provisioning"
	"github.com/rancher/rancher/tests/framework/pkg/clientbase"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Client struct {
	Management    *management.Client
	Provisioning  *provisioning.Client
	RancherConfig *Config
	restcConfig   *rest.Config
	session       *session.Session
}

func NewClient(bearerToken string, rancherConfig *Config, session *session.Session) (*Client, error) {
	c := &Client{
		RancherConfig: rancherConfig,
	}

	var err error
	restConfig := newRestConfig(bearerToken, rancherConfig)
	c.restcConfig = restConfig
	c.session = session
	c.Management, err = management.NewClient(clientOpts(restConfig, c.RancherConfig))
	if err != nil {
		return nil, err
	}

	c.Management.Ops.Session = session

	provClient, err := provisioning.NewForConfig(restConfig, session)
	if err != nil {
		return nil, err
	}

	c.Provisioning = provClient

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

// GetRancherDynamicClient is a helper function that instantiates a dynamic client to communicate with the rancher host.
func (c *Client) GetRancherDynamicClient() (dynamic.Interface, error) {
	dynamic, err := frameworkDynamic.NewForConfig(c.session, c.restcConfig)
	if err != nil {
		return nil, err
	}
	return dynamic, nil
}
