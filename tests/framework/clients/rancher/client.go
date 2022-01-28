package rancher

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	frameworkDynamic "github.com/rancher/rancher/tests/framework/clients/dynamic"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
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

// Client is the main rancher Client object that gives an end user access to the Provisioning and Management
// clients in order to create resources on rancher
type Client struct {
	Management    *management.Client
	Provisioning  *provisioning.Client
	Catalog       *catalog.Client
	RancherConfig *Config
	restConfig    *rest.Config
	Session       *session.Session
}

// NewClient is the constructor to the intializing a rancher Client. It takes a bearer token and session.Session. If bearer token is not provided,
// the bearer token provided in the configuration file is used.
func NewClient(bearerToken string, session *session.Session) (*Client, error) {
	rancherConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, rancherConfig)

	if bearerToken == "" {
		bearerToken = rancherConfig.AdminToken
	}

	c := &Client{
		RancherConfig: rancherConfig,
	}

	session.CleanupEnabled = *rancherConfig.Cleanup

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

	catalogClient, err := catalog.NewForConfig(restConfig, session)
	if err != nil {
		return nil, err
	}

	c.Catalog = catalogClient

	return c, nil
}

// newRestConfig is a constructor that sets ups rest.Config the configuration used by the Provisioning client.
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

// clientOpts is a constructor that sets ups clientbase.ClientOpts the configuration used by the Management client.
func clientOpts(restConfig *rest.Config, rancherConfig *Config) *clientbase.ClientOpts {
	return &clientbase.ClientOpts{
		URL:      fmt.Sprintf("https://%s/v3", rancherConfig.Host),
		TokenKey: restConfig.BearerToken,
		Insecure: restConfig.Insecure,
		CACerts:  rancherConfig.CACerts,
	}
}

// AsUser accepts a user object, and then creates a token for said `user`. Then it instantiates and returns a Client using the token created
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

// WithSession accepts a session.Session and instantiates a new Client to reference this new session.Session. The main purpose is to use it
// when created "sub sessions" when tracking resources created at a test case scope
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

// GetDownStreamClusterClient is a helper function that instantiates a dynamic client to communicate with a specific cluster.
func (c *Client) GetDownStreamClusterClient(clusterID string) (dynamic.Interface, error) {
	c.restConfig.Host = fmt.Sprintf("https://%s/k8s/clusters/%s", c.restConfig.Host, clusterID)

	dynamic, err := frameworkDynamic.NewForConfig(c.Session, c.restConfig)
	if err != nil {
		return nil, err
	}
	return dynamic, nil
}

// GetManagementWatchInterface is a functions used to get a watch.Interface from a resource created by the Management Client.
// As is the Management resources do not have a watch.Interface, so therefore, the dynamic Client is used to get the watch.Interface.
// The `schemaType` is a string that is found in different Management clients packages. Ex) management.ProjectType
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
