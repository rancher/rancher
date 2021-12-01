package rancher

import (
	"context"
	"errors"
	"fmt"

	frameworkDynamic "github.com/rancher/rancher/tests/framework/clients/dynamic"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher/provisioning"
	"github.com/rancher/rancher/tests/framework/pkg/clientbase"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Client struct {
	Management    *management.Client
	Provisioning  *provisioning.Client
	RancherConfig *Config
	restConfig    *rest.Config
	Session       *session.Session
}

func NewClient(bearerToken string, session *session.Session) (*Client, error) {
	rancherConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, rancherConfig)

	if bearerToken == "" {
		bearerToken = rancherConfig.AdminToken
	}

	c := &Client{
		RancherConfig: rancherConfig,
	}

	var err error
	restConfig := newRestConfig(bearerToken, rancherConfig)
	c.restConfig = restConfig
	c.Session = session
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
		Host:        rancherConfig.Host,
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: *rancherConfig.Insecure,
			CAFile:   rancherConfig.CAFile,
		},
	}
}

func clientOpts(restConfig *rest.Config, rancherConfig *Config) *clientbase.ClientOpts {
	return &clientbase.ClientOpts{
		URL:      fmt.Sprintf("https://%s/v3", rancherConfig.Host),
		TokenKey: restConfig.BearerToken,
		Insecure: restConfig.Insecure,
		CACerts:  rancherConfig.CACerts,
	}
}

func (c *Client) AsUser(user *management.User) (*Client, error) {
	token := &management.Token{
		UserID: user.ID,
	}

	returnedToken, err := c.Management.Token.Create(token)
	if err != nil {
		return nil, err
	}

	return NewClient(returnedToken.Token, c.Session)
}

func (c *Client) WithSession(session *session.Session) (*Client, error) {
	return NewClient(c.restConfig.BearerToken, session)
}

// GetRancherDynamicClient is a helper function that instantiates a dynamic client to communicate with the rancher host.
func (c *Client) GetRancherDynamicClient() (dynamic.Interface, error) {
	dynamic, err := frameworkDynamic.NewForConfig(c.Session, c.restConfig)
	if err != nil {
		return nil, err
	}
	return dynamic, nil
}

func (c *Client) GetManagementWatchInterface(schemaType string, opts metav1.ListOptions) (watch.Interface, error) {
	schemaResource, ok := c.Management.APIBaseClient.Ops.Types[schemaType]
	if !ok {
		return nil, errors.New("Unknown schema type [" + schemaType + "]")
	}

	groupVersionResource := schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: schemaResource.PluralName,
	}
	dynamicClient, err := c.GetRancherDynamicClient()
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(groupVersionResource).Watch(context.TODO(), opts)
}
