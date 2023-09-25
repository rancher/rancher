package openldapauth

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OpenLdapTest struct {
	suite.Suite
	testUser *management.User
	client   *rancher.Client
	project  *management.Project
	session  *session.Session
}

// APIResponse represents the outcome of the SendAPICall function
type APIResponse struct {
	StatusCode int
	Body       string
}

func (d *OpenLdapTest) SetupSuite() {
	testSession := session.NewSession()

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(d.T(), err)

	d.project = testProject
	/*
		upgradeConfig := new(Config2)
		config.LoadConfig(ConfigurationFileKey, upgradeConfig)
		fmt.Print(upgradeConfig.OpenLdapUser)
		fmt.Print(upgradeConfig.OpenLdapUserPass) */

	//enabled := true

	/*user := &management.User{
		Username: "testusername",
		Password: "passwordpasswordd",
		Name:     "displayname",
		Enabled:  &enabled,
	}*/

	//client.AsUser(user)

}
func (d *OpenLdapTest) TearDownSuite() {
	d.session.Cleanup()
}

func (d *OpenLdapTest) TestEnableOpenLDAP() {
	testSession := session.NewSession()
	d.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	upgradeConfig := new(Config2)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)
	//MakeOpenLdapAPICALL(url, token, body)
	fmt.Print(upgradeConfig.OpenLdapUser)
	fmt.Print(upgradeConfig.OpenLdapUserPass)
	fmt.Print(client)
	fmt.Print(d.testUser)

	/*
		rancherClient, err := rancher.NewRancherClient()
		require.NoError(t, err)

		authConfig, err := CreateOpenLDAPAuthConfig(rancherClient)
		require.NoError(t, err)
	*/
	/*
		assert.Equal(t, openLdapConfigNameBase, authConfig.Name)
		assert.NotNil(t, authConfig.OpenLDAPCredentialConfig)
		assert.Equal(t, []string{"openldapqa.qa.rancher.space"}, authConfig.OpenLDAPCredentialConfig.Servers)
		assert.Equal(t, "cn=admin,dc=qa,rancher,dc=space", authConfig.OpenLDAPCredentialConfig.ServiceAccountDistinguishedName)
	*/
}

func (d *OpenLdapTest) MakeOpenLdapAPICall() {
	host := "ron276c" // Consider fetching this from `d` if it's a property of OpenLdapTest.
	url := fmt.Sprintf("https://%s.qa.rancher.space/v3-public/localProviders/local?action=login", host)
	token := "token-sss6h:bsz527k7jcpr8bbjmw22wlg5w8vqzlq5w9snwpzfx7xzb8fm6hqqsp" // Similarly, fetch from `d` if required.

	// Adjust the body as per your requirements. I'm using the one from the example.
	body := []byte(`{"description": "postman", "responseType": "token", "username": "admin", "password": "N7q-*fs+Ut&Wb_Y"}`)

	resp, err := MakeOpenLdapAPICALL(url, token, body)
	if err != nil {
		// Handle error
		fmt.Println("Error:", err)
		return
	}

	// Inspect the APIResponse
	if resp.StatusCode == http.StatusOK {
		fmt.Println("API call was successful!")
	} else {
		fmt.Printf("API call failed with status code: %d\n", resp.StatusCode)
	}
	fmt.Println("Response Body:", resp.Body)
}

// SendAPICall sends a POST request to a given URL with a provided bearer token and body.
// It returns an APIResponse containing the status code and response body.
func MakeOpenLdapAPICALL(url, token string, body []byte) (*APIResponse, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}, nil
}

func APIFunction() {
	host := "ron276c"
	url := fmt.Sprintf("https://%s.qa.rancher.space/v3-public/localProviders/local?action=login", host)
	token := "token-sss6h:bsz527k7jcpr8bbjmw22wlg5w8vqzlq5w9snwpzfx7xzb8fm6hqqsp"
	body := []byte(`{"description": "postman", "responseType": "token", "username": "admin", "password": "N7q-*fs+Ut&Wb_Y"}`)

	resp, err := MakeOpenLdapAPICALL(url, token, body)
	if err != nil {
		// Handle error
		fmt.Println("Error:", err)
		return
	}

	// Inspect the APIResponse
	if resp.StatusCode == http.StatusOK {
		fmt.Println("API call was successful!")
	} else {
		fmt.Printf("API call failed with status code: %d\n", resp.StatusCode)
	}
	fmt.Println("Response Body:", resp.Body)
}

func TestOpenLdapTestSuite(t *testing.T) {
	suite.Run(t, new(OpenLdapTest))
}

//move credentials from code to config
//rename/polish struct
//check api calls for what's sent/response
//check network and get info for structs
