package rancher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	frameworkDynamic "github.com/rancher/rancher/tests/framework/clients/dynamic"
	"github.com/rancher/rancher/tests/framework/clients/ec2"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"

	kubeProvisioning "github.com/rancher/rancher/tests/framework/clients/provisioning"
	kubeRKE "github.com/rancher/rancher/tests/framework/clients/rke"
	"github.com/rancher/rancher/tests/framework/pkg/clientbase"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
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
	// Client used to access management.cattle.io v3 API resources
	Management *management.Client
	// Client used to access Steve v1 API resources
	Steve *v1.Client
	// Client used to access catalog.cattle.io v1 API resources (apps, charts, etc.)
	Catalog *catalog.Client
	// Config used to test against a rancher instance
	RancherConfig *Config
	// Session is the session object used by the client to track all the resources being created by the client.
	Session *session.Session
	// Flags is the environment flags used by the client to test selectively against a rancher instance.
	Flags      *environmentflag.EnvironmentFlags
	restConfig *rest.Config
}

// NewClient is the constructor to the initializing a rancher Client. It takes a bearer token and session.Session. If bearer token is not provided,
// the bearer token provided in the configuration file is used.
func NewClient(bearerToken string, session *session.Session) (*Client, error) {
	rancherConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, rancherConfig)

	environmentFlags := environmentflag.NewEnvironmentFlags()
	environmentflag.LoadEnvironmentFlags(environmentflag.ConfigurationFileKey, environmentFlags)

	if bearerToken == "" {
		bearerToken = rancherConfig.AdminToken
	}

	c := &Client{
		RancherConfig: rancherConfig,
		Flags:         &environmentFlags,
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

	c.Steve, err = v1.NewClient(clientOptsV1(restConfig, c.RancherConfig))
	if err != nil {
		return nil, err
	}

	c.Steve.Ops.Session = session

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

// clientOptsV1 is a constructor that sets ups clientbase.ClientOpts the configuration used by the v1 Rancher clients.
func clientOptsV1(restConfig *rest.Config, rancherConfig *Config) *clientbase.ClientOpts {
	return &clientbase.ClientOpts{
		URL:      fmt.Sprintf("https://%s/v1", rancherConfig.Host),
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

	byteContent, err := io.ReadAll(resp.Body)
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

// ReLogin reinstantiates a Client to update its API schema. This function would be used for a non admin user that needs to be
// "reloaded" inorder to have updated permissions for certain resources.
func (c *Client) ReLogin() (*Client, error) {
	return NewClient(c.restConfig.BearerToken, c.Session)
}

// WithSession accepts a session.Session and instantiates a new Client to reference this new session.Session. The main purpose is to use it
// when created "sub sessions" when tracking resources created at a test case scope.
func (c *Client) WithSession(session *session.Session) (*Client, error) {
	return NewClient(c.restConfig.BearerToken, session)
}

// GetClusterCatalogClient is a function that takes a clusterID and instantiates a catalog client to directly communicate with that specific cluster.
func (c *Client) GetClusterCatalogClient(clusterID string) (*catalog.Client, error) {
	restConfig := *c.restConfig
	restConfig.Host = fmt.Sprintf("https://%s/k8s/clusters/%s", c.restConfig.Host, clusterID)

	catalogClient, err := catalog.NewForConfig(&restConfig, c.Session)
	if err != nil {
		return nil, err
	}

	return catalogClient, nil
}

// GetRancherDynamicClient is a helper function that instantiates a dynamic client to communicate with the rancher host.
func (c *Client) GetRancherDynamicClient() (dynamic.Interface, error) {
	dynamic, err := frameworkDynamic.NewForConfig(c.Session, c.restConfig)
	if err != nil {
		return nil, err
	}
	return dynamic, nil
}

// GetKubeAPIProvisioningClient is a function that instantiates a provisioning client that communicates with the Kube API of a cluster
func (c *Client) GetKubeAPIProvisioningClient() (*kubeProvisioning.Client, error) {
	provClient, err := kubeProvisioning.NewForConfig(c.restConfig, c.Session)
	if err != nil {
		return nil, err
	}

	return provClient, nil
}

// GetKubeAPIRKEClient is a function that instantiates a rke client that communicates with the Kube API of a cluster
func (c *Client) GetKubeAPIRKEClient() (*kubeRKE.Client, error) {
	rkeClient, err := kubeRKE.NewForConfig(c.restConfig, c.Session)
	if err != nil {
		return nil, err
	}

	return rkeClient, nil
}

// GetDownStreamClusterClient is a helper function that instantiates a dynamic client to communicate with a specific cluster.
func (c *Client) GetDownStreamClusterClient(clusterID string) (dynamic.Interface, error) {
	restConfig := *c.restConfig
	restConfig.Host = fmt.Sprintf("https://%s/k8s/clusters/%s", c.restConfig.Host, clusterID)

	dynamic, err := frameworkDynamic.NewForConfig(c.Session, &restConfig)
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

// GetEC2Client is a helper function that instantiates an aws ec2 client to communicate with the ec2 instances on aws.
func (c *Client) GetEC2Client() (*ec2.Client, error) {
	return ec2.NewClient()
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
	err = c.doAction("/v3-public/localProviders/local", "login", bodyContent, token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// IsConnected is a helper function that pings rancher ping endpoint with the management, steve and rest clients.
// Returns boolean value depending on if all the clients are able to get pong respond.
func (c *Client) IsConnected() (isConnected bool, err error) {
	mngmntPong, err := c.ping(c.Management.APIBaseClient.Ops.Client)
	if err != nil {
		return
	}

	stevePong, err := c.ping(c.Steve.APIBaseClient.Ops.Client)
	if err != nil {
		return
	}

	restHTTP, err := rest.HTTPClientFor(c.restConfig)
	if err != nil {
		return false, err
	}
	restPong, err := c.ping(restHTTP)
	if err != nil {
		return
	}

	isConnected = mngmntPong == stevePong == restPong != isConnected
	return
}

// ping uses http client to ping rancher ping endpoint, returns boolean value if pong is returned
func (c *Client) ping(httpClient *http.Client) (bool, error) {
	url := "https://" + c.restConfig.Host + "/ping"
	pong := "pong"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Add("Authorization", "Bearer "+c.restConfig.BearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}

	if resp != nil {
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		if len(bodyBytes) > 0 {
			return string(bodyBytes) == pong, err
		}
	}

	return false, err
}
