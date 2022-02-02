package rancher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	frameworkDynamic "github.com/rancher/rancher/tests/framework/clients/dynamic"
	"github.com/rancher/rancher/tests/framework/clients/ec2"
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
	"k8s.io/client-go/tools/clientcmd"
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

// NewClient is the constructor to the initializing a rancher Client. It takes a bearer token and session.Session. If bearer token is not provided,
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

// doAction is used to post an action to an endpoint, and marshal the response into the output parameter.
func (c *Client) doAction(endpoint, action string, body []byte, output interface{}) error {
	url := "https://" + c.restConfig.Host + endpoint + "?action=" + action
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.restConfig.BearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Management.APIBaseClient.Ops.Client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return httperror.NewAPIErrorLong(resp.StatusCode, resp.Status, url)
	}

	byteContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(byteContent) > 0 {
		err = json.Unmarshal(byteContent, output)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("received empty response")
}

// AsUser accepts a user object, and then creates a token for said `user`. Then it instantiates and returns a Client using the token created.
// This function uses the login action, and user must have a correct username and password combination.
func (c *Client) AsUser(user *management.User) (*Client, error) {
	returnedToken, err := c.login(user)
	if err != nil {
		return nil, err
	}

	return NewClient(returnedToken.Token, c.Session)
}

// WithSession accepts a session.Session and instantiates a new Client to reference this new session.Session. The main purpose is to use it
// when created "sub sessions" when tracking resources created at a test case scope.
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

// SwitchContext is a helper function that changes the current context to `context` and instantiates a dynamic client
func (c *Client) SwitchContext(context string, clientConfig *clientcmd.ClientConfig) (dynamic.Interface, error) {
	overrides := clientcmd.ConfigOverrides{CurrentContext: context}

	rawConfig, err := (*clientConfig).RawConfig()
	if err != nil {
		return nil, err
	}

	updatedConfig := clientcmd.NewNonInteractiveClientConfig(rawConfig, rawConfig.CurrentContext, &overrides, (*clientConfig).ConfigAccess())

	restConfig, err := updatedConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	dynamic, err := frameworkDynamic.NewForConfig(c.Session, restConfig)
	if err != nil {
		return nil, err
	}

	return dynamic, nil
}

func (c *Client) GetEC2Client() (*ec2.EC2Client, error) {
	return ec2.NewEC2Client()
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

// login uses the local authentication provider to authenticate a user and return the subsequent token.
func (c *Client) login(user *management.User) (*management.Token, error) {
	token := &management.Token{}
	bodyContent, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: user.Username,
		Password: user.Password,
	})
	if err != nil {
		return nil, err
	}
	err = c.doAction("/v3-public/localproviders/local", "login", bodyContent, token)
	if err != nil {
		return nil, err
	}

	return token, nil
}
