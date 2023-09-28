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

// main test struct - includes test suite, user, rancher client, project and a session
// this is used for the end-to-end tests
type OpenLdapTest struct {
	suite.Suite
	testUser *management.User
	client   *rancher.Client
	project  *management.Project
	session  *session.Session
}

// APIResponse represents the outcome of the API function - build func for 200 401
type APIResponse struct {
	StatusCode int
	Body       string
}

// executes before any tests; new session, client and project created (testproject)
// will uncomment code to read config2 - not yet implemented
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

// cleanup the session, ensure any resources are released
func (d *OpenLdapTest) TearDownSuite() {
	d.session.Cleanup()
}

// initializes new session and rancher client (?)
// commented out some config reading and assertions i was trying
func (d *OpenLdapTest) TestEnableOpenLDAP() {
	testSession := session.NewSession()
	d.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	upgradeConfig := new(Config2)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)
	//sendAPICall(url, token, body)
	fmt.Print(upgradeConfig.OpenLdapUser)
	fmt.Print(upgradeConfig.OpenLdapUserPass)
	fmt.Print(client)
	fmt.Print(d.testUser)
}

// trying to build a body
func prepareRequestBody(url string) []byte {
	requestBody := `
	{
		"enabled": true,
		"ldapConfig": {
			"actions": {
				"testAndApply": "https://ron280alpha1b.qa.rancher.space/v3/openLdapConfigs/openldap?action=testAndApply"
			},
			"annotations": {
				"management.cattle.io/auth-provider-cleanup": "rancher-locked"
			},
			"baseType": "authConfig",
			"connectionTimeout": 5000,
			"creatorId": null,
			"enabled": true,
			"groupDNAttribute": "entryDN",
			"groupMemberMappingAttribute": "member",
			"groupMemberUserAttribute": "entryDN",
			"groupNameAttribute": "cn",
			"groupObjectClass": "groupOfNames",
			"groupSearchAttribute": "cn",
			"id": "openldap",
			"labels": {
				"cattle.io/creator": "norman"
			},
			"links": {
				"self": "https://ron280alpha1b.qa.rancher.space/v3/openLdapConfigs/openldap",
				"update": "https://ron280alpha1b.qa.rancher.space/v3/openLdapConfigs/openldap"
			},
			"name": "openldap",
			"nestedGroupMembershipEnabled": false,
			"port": 389,
			"starttls": false,
			"tls": false,
			"type": "openLdapConfig",
			"userDisabledBitMask": 0,
			"userLoginAttribute": "uid",
			"userMemberAttribute": "memberOf",
			"userNameAttribute": "cn",
			"userObjectClass": "inetOrgPerson",
			"userSearchAttribute": "uid|sn|givenName",
			"uuid": "c30859dd-f103-446b-ad81-9633f2da0438",
			"__clone": true,
			"servers": [
				"openldapqa.qa.rancher.space"
			],
			"accessMode": "unrestricted",
			"disabledStatusBitmask": 0,
			"serviceAccountDistinguishedName": "cn=admin,dc=qa,dc=rancher,dc=space",
			"serviceAccountPassword": "cattle@123",
			"userSearchBase": "dc=qa,dc=rancher,dc=space",
			"groupSearchBase": ""
		},
		"username": "testuser1",
		"password": "Tacos86!"
	}
    `
	return []byte(requestBody)
}

// new code to invoke makeopenldapapicall()
func (d *OpenLdapTest) TestOpenLdapAPI() {
	url := "https://ron280alpha1b.qa.rancher.space/v3/openLdapConfigs/openldap?action=testAndApply"
	token := "token-ttnmk:mvkn4csbfsk48tbdfbqg5r6pqp6rxvc8rrv4x46n72jcssww52l7dq"
	//body := prepareRequestBody(url)
	//resp, err := d.EnableOpenLdap(url, token, body)
	resp, err := d.DisableOpenLDAP(url, token)
	//body := prepareRequestBody(url)
	//resp, err := d.EnableOpenLdap(url, token, body)
	// Assert there's no error
	require.NoError(d.T(), err)

	// Assert that the response code is 200 OK
	require.Equal(d.T(), http.StatusOK, resp.StatusCode)

	// You can also assert on other parts of the response as needed
	// require.Contains(d.T(), resp.Body, "Expected content in body")
}

// attempting api call
func (d *OpenLdapTest) EnableOpenLdap(url string, token string, body []byte) (*APIResponse, error) {

	// Call the SendAPICall function using the passed arguments
	resp, err := SendAPICall(url, token, body)
	if err != nil {
		// Handle error and return
		fmt.Println("Error:", err)
		return nil, err
	}

	// Inspect the APIResponse and print
	if resp.StatusCode == http.StatusOK {
		fmt.Println("API call was successful!")
	} else {
		fmt.Printf("API call failed with status code: %d\n", resp.StatusCode)
		fmt.Println("Response Body:", resp.Body)
	}
	return resp, nil
}

// this code tries to disable the auth provider with a specific api call
func (d *OpenLdapTest) DisableOpenLDAP(host string, token string) (*APIResponse, error) {
	// Construct the URL
	//host := "ron280alpha1b.qa.rancher.space"
	url := "https://ron280alpha1b.qa.rancher.space/v3/openLdapConfigs/openldap?action=disable"

	// Create the request body
	body := []byte(`{"action": "disable"}`)

	// Use the SendAPICall function to send the request
	return SendAPICall(url, token, body)
}

// SendAPICall sends a POST request to a given URL with a provided bearer token and body.
// It returns an APIResponse containing the status code and response body.
// This is a generic function to make a POST API call to a given URL with a bearer token and JSON body.
func SendAPICall(url, token string, body []byte) (*APIResponse, error) {
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

func TestOpenLdapTestSuite(t *testing.T) {
	suite.Run(t, new(OpenLdapTest))
}

//move credentials from code to config
//rename/polish struct
//check api calls for what's sent/response
//check network and get info for structs
