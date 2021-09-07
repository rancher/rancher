package clients

import (
	"github.com/rancher/norman/clientbase"
	projectClient "github.com/rancher/rancher/pkg/client/generated/project/v3"
	provisionClient "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	coreV1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/tests/automation-framework/aws"
	"github.com/rancher/rancher/tests/automation-framework/config"
	managementClient "github.com/rancher/rancher/tests/automation-framework/management"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Client struct {
	Provisioning *provisionClient.ProvisioningV1Client
	Management   *managementClient.Client
	Project      *projectClient.Client
	EC2          *aws.EC2Client
	Core         coreV1.Interface
	Dynamic      dynamic.Interface
	RestConfig   *rest.Config
}

// NewClient is returns a larger client wrapping individual api clients.
func NewClient(restConfig *rest.Config) (*Client, error) {
	configuration := config.GetInstance()

	c := &Client{
		RestConfig: restConfig,
	}

	err := c.newProvisioningClient()
	if err != nil {
		return nil, err
	}

	err = c.newDynamic()
	if err != nil {
		return nil, err
	}

	err = c.newCoreV1Client()
	if err != nil {
		return nil, err
	}

	err = c.newManagementClient()
	if err != nil {
		return nil, err
	}

	err = c.newProjectClient()
	if err != nil {
		return nil, err
	}

	if configuration.GetAWSAccessKeyID() != "" && configuration.GetAWSSecretAccessKey() != "" {
		err = c.newEC2Client()
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// NewRestConfig is the config used the various clients
func NewRestConfig(bearerToken string) *rest.Config {
	configuration := config.GetInstance()
	return &rest.Config{
		Host:        configuration.GetCattleTestURL(),
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: configuration.GetInsecure(),
			CAFile:   configuration.GetCAFile(),
		},
	}
}

func clientOpts(restConfig *rest.Config) *clientbase.ClientOpts {
	configuration := config.GetInstance()
	return &clientbase.ClientOpts{
		URL:      restConfig.Host + "v3",
		TokenKey: restConfig.BearerToken,
		Insecure: restConfig.Insecure,
		CACerts:  configuration.GetCAFile(),
	}
}

func (c *Client) newProvisioningClient() error {
	client, err := provisionClient.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	c.Provisioning = client
	return nil
}

func (c *Client) newDynamic() error {
	dynamic, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	c.Dynamic = dynamic
	return nil
}

func (c *Client) newCoreV1Client() error {
	core, err := coreV1.NewForConfig(*c.RestConfig)
	if err != nil {
		return err
	}

	c.Core = core
	return nil
}

func (c *Client) newManagementClient() error {
	clientOpts := clientOpts(c.RestConfig)
	client, err := managementClient.NewClient(clientOpts)
	if err != nil {
		return err
	}
	c.Management = client
	return nil
}

func (c *Client) newProjectClient() error {
	clientOpts := clientOpts(c.RestConfig)
	client, err := projectClient.NewClient(clientOpts)
	if err != nil {
		return err
	}

	c.Project = client
	return nil
}

func (c *Client) newEC2Client() error {
	newClient, err := aws.NewEC2Client()
	if err != nil {
		return err
	}

	c.EC2 = newClient

	return nil
}
